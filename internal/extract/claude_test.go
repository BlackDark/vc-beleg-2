package extract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
)

func ptr[T any](v T) *T { return &v }

func TestConvert(t *testing.T) {
	ex := convert(answer{Date: ptr("2026-10-05"), Total: ptr(7.89), Merchant: ptr(" REWE "), Uncertain: []string{"merchant"}})
	if ex.Date != "2026-10-05" || ex.Total != 789 || ex.Merchant != "REWE" || !slices.Equal(ex.Uncertain, []string{"merchant"}) {
		t.Fatalf("convert = %+v", ex)
	}
	ex = convert(answer{Date: ptr("05.10.2026"), Total: ptr(-1.0)})
	if ex.Date != "" || ex.Total != 0 || !slices.Contains(ex.Uncertain, "date") || !slices.Contains(ex.Uncertain, "total") {
		t.Fatalf("implausible values kept: %+v", ex)
	}
	if ex := convert(answer{Total: ptr(0.1 + 0.2)}); ex.Total != 30 {
		t.Fatalf("rounding: %d", ex.Total)
	}
}

// fakeAPI answers like the Messages API and records the last request.
func fakeAPI(t *testing.T, stopReason, text string) (*httptest.Server, *map[string]any, *http.Header) {
	t.Helper()
	var body map[string]any
	var header http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
			"stop_reason": stopReason, "content": []map[string]any{{"type": "text", "text": text}},
			"usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &body, &header
}

func TestExtractSendsFallback(t *testing.T) {
	srv, body, header := fakeAPI(t, "end_turn", `{"date":"2026-10-05","total":7.89,"merchant":"REWE","uncertain":[]}`)
	c := New("key", "", true, option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	ex, err := c.Extract(context.Background(), "image/png", []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	if ex.Date != "2026-10-05" || ex.Total != 789 || ex.Merchant != "REWE" {
		t.Fatalf("extraction = %+v", ex)
	}
	if (*body)["fallbacks"] != "default" || (*body)["model"] != DefaultModel {
		t.Fatalf("request = %v", *body)
	}
	if !strings.Contains(header.Get("anthropic-beta"), "server-side-fallback-2026-07-01") {
		t.Fatalf("beta header = %q", header.Get("anthropic-beta"))
	}
}

func TestExtractWithoutFallback(t *testing.T) {
	srv, body, _ := fakeAPI(t, "end_turn", `{"date":null,"total":null,"merchant":null,"uncertain":[]}`)
	c := New("key", "claude-sonnet-5-5", false, option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	if _, err := c.Extract(context.Background(), "application/pdf", []byte("%PDF-1")); err != nil {
		t.Fatal(err)
	}
	if _, ok := (*body)["fallbacks"]; ok || (*body)["model"] != "claude-sonnet-5-5" {
		t.Fatalf("request = %v", *body)
	}
}

func TestExtractRefusal(t *testing.T) {
	srv, _, _ := fakeAPI(t, "refusal", "")
	c := New("key", "", true, option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
	if _, err := c.Extract(context.Background(), "image/png", []byte("png")); err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("err = %v", err)
	}
}
