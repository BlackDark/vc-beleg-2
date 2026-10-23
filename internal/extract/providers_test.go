package extract

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeJSON answers every request with status and reply and records the
// last request.
type fakeJSON struct {
	srv    *httptest.Server
	path   string
	header http.Header
	body   map[string]any
	calls  atomic.Int32
}

func newFakeJSON(t *testing.T, status func(call int) int, reply any) *fakeJSON {
	t.Helper()
	f := &fakeJSON{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := int(f.calls.Add(1))
		f.path, f.header = r.URL.Path, r.Header.Clone()
		f.body = nil
		_ = json.NewDecoder(r.Body).Decode(&f.body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status(call))
		_ = json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func ok(int) int { return http.StatusOK }

func geminiReply(finish, text string) map[string]any {
	return map[string]any{"candidates": []any{map[string]any{
		"finishReason": finish,
		"content":      map[string]any{"parts": []any{map[string]any{"text": text}}},
	}}}
}

func openAIReply(finish, content, refusal string) map[string]any {
	msg := map[string]any{"role": "assistant", "content": content}
	if refusal != "" {
		msg["refusal"] = refusal
	}
	return map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": msg}}}
}

func TestGemini(t *testing.T) {
	f := newFakeJSON(t, ok, geminiReply("STOP", `{"date":"2026-10-05","total":7.89,"merchant":"REWE","uncertain":["total"]}`))
	ex, err := NewGemini("gkey", "", f.srv.URL).Extract(context.Background(), "application/pdf", []byte("%PDF-1"))
	if err != nil {
		t.Fatal(err)
	}
	if ex.Date != "2026-10-05" || ex.Total != 789 || ex.Merchant != "REWE" || ex.Uncertain[0] != "total" {
		t.Fatalf("extraction = %+v", ex)
	}
	if f.path != "/v1beta/models/gemini-flash-latest:generateContent" || f.header.Get("X-Goog-Api-Key") != "gkey" {
		t.Fatalf("request %s, key %q", f.path, f.header.Get("X-Goog-Api-Key"))
	}
	parts := f.body["contents"].([]any)[0].(map[string]any)["parts"].([]any)
	inline := parts[0].(map[string]any)["inlineData"].(map[string]any)
	if inline["mimeType"] != "application/pdf" || inline["data"] != "JVBERi0x" {
		t.Fatalf("inline data = %v", inline)
	}
	if f.body["generationConfig"].(map[string]any)["responseMimeType"] != "application/json" {
		t.Fatalf("generation config = %v", f.body["generationConfig"])
	}
}

func TestGeminiDeclined(t *testing.T) {
	for name, reply := range map[string]any{
		"safety":  geminiReply("SAFETY", ""),
		"blocked": map[string]any{"promptFeedback": map[string]any{"blockReason": "OTHER"}},
	} {
		f := newFakeJSON(t, ok, reply)
		if _, err := NewGemini("k", "", f.srv.URL).Extract(context.Background(), "image/png", []byte("png")); !errors.Is(err, errDeclined) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestOpenAI(t *testing.T) {
	f := newFakeJSON(t, ok, openAIReply("stop", "```json\n{\"date\":\"2026-10-05\",\"total\":7.89,\"merchant\":\"REWE\",\"uncertain\":[]}\n```", ""))
	ex, err := NewOpenAI("okey", "google/gemini-flash", f.srv.URL+"/api/v1/").Extract(context.Background(), "image/jpeg", []byte("jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if ex.Date != "2026-10-05" || ex.Total != 789 || ex.Merchant != "REWE" {
		t.Fatalf("extraction = %+v", ex)
	}
	if f.path != "/api/v1/chat/completions" || f.header.Get("Authorization") != "Bearer okey" || f.body["model"] != "google/gemini-flash" {
		t.Fatalf("request %s %q %v", f.path, f.header.Get("Authorization"), f.body["model"])
	}
	content := f.body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	img := content[0].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(img, "data:image/jpeg;base64,") {
		t.Fatalf("image = %q", img)
	}
	if f.body["response_format"].(map[string]any)["type"] != "json_schema" {
		t.Fatalf("response format = %v", f.body["response_format"])
	}
}

func TestOpenAIPDFAndNoKey(t *testing.T) {
	f := newFakeJSON(t, ok, openAIReply("stop", `{"date":null,"total":null,"merchant":null,"uncertain":[]}`, ""))
	if _, err := NewOpenAI("", "llava", f.srv.URL).Extract(context.Background(), "application/pdf", []byte("%PDF-1")); err != nil {
		t.Fatal(err)
	}
	content := f.body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	file := content[0].(map[string]any)["file"].(map[string]any)
	if file["file_data"] != "data:application/pdf;base64,JVBERi0x" || f.header.Get("Authorization") != "" {
		t.Fatalf("file = %v, auth %q", file, f.header.Get("Authorization"))
	}
}

func TestOpenAIDeclined(t *testing.T) {
	for name, reply := range map[string]any{
		"refusal": openAIReply("stop", "", "I can't help with that."),
		"filter":  openAIReply("content_filter", "", ""),
	} {
		f := newFakeJSON(t, ok, reply)
		if _, err := NewOpenAI("k", "m", f.srv.URL).Extract(context.Background(), "image/png", []byte("png")); !errors.Is(err, errDeclined) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestRetries(t *testing.T) {
	saved := retryWaits
	retryWaits = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { retryWaits = saved })

	// 429, 503, then success.
	f := newFakeJSON(t, func(call int) int {
		switch call {
		case 1:
			return http.StatusTooManyRequests
		case 2:
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	}, openAIReply("stop", `{"date":null,"total":7.5,"merchant":null,"uncertain":[]}`, ""))
	ex, err := NewOpenAI("k", "m", f.srv.URL).Extract(context.Background(), "image/png", []byte("png"))
	if err != nil || ex.Total != 750 || f.calls.Load() != 3 {
		t.Fatalf("ex = %+v, err = %v, calls = %d", ex, err, f.calls.Load())
	}

	// Client errors are not retried and keep the body out of the error.
	f = newFakeJSON(t, func(int) int { return http.StatusBadRequest }, map[string]any{"error": "receipt text"})
	_, err = NewGemini("k", "", f.srv.URL).Extract(context.Background(), "image/png", []byte("png"))
	if err == nil || err.Error() != "api: status 400" || f.calls.Load() != 1 {
		t.Fatalf("err = %v, calls = %d", err, f.calls.Load())
	}
}
