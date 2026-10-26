package extract

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
)

type pinger interface{ Ping(context.Context) error }

func TestOpenAIConnectionPing(t *testing.T) {
	f := newFakeJSON(t, ok, openAIReply("stop", "pong", ""))
	reader := NewOpenAI("key", "configured-model", f.srv.URL)
	p, ok := any(reader).(pinger)
	if !ok {
		t.Fatal("OpenAI does not support a connection ping")
	}
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(f.body)
	if f.body["model"] != "configured-model" || f.body["reasoning_effort"] != "low" || strings.Contains(string(raw), "data:") || strings.Contains(string(raw), "image_url") || strings.Contains(string(raw), "json_schema") {
		t.Fatalf("ping must be text-only with configured model/effort: %s", raw)
	}
	if f.header.Get("Authorization") != "Bearer key" {
		t.Fatal("configured auth not used")
	}
}

func TestGeminiConnectionPing(t *testing.T) {
	f := newFakeJSON(t, ok, geminiReply("STOP", "pong"))
	p, ok := any(NewGemini("key", "configured-model", f.srv.URL)).(pinger)
	if !ok {
		t.Fatal("Gemini does not support a connection ping")
	}
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(f.body)
	if strings.Contains(string(raw), "inlineData") || !strings.Contains(string(raw), "pong") || f.path != "/v1beta/models/configured-model:generateContent" {
		t.Fatalf("ping must contain only text: %s at %s", raw, f.path)
	}
}

func TestClaudeConnectionPing(t *testing.T) {
	srv, body, _ := fakeAPI(t, "end_turn", "pong")
	p, ok := any(NewClaude("key", "configured-model", true, option.WithBaseURL(srv.URL), option.WithMaxRetries(0))).(pinger)
	if !ok {
		t.Fatal("Claude does not support a connection ping")
	}
	if err := p.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(body)
	if (*body)["model"] != "configured-model" || strings.Contains(string(raw), "base64") || strings.Contains(string(raw), "document") || strings.Contains(string(raw), "image") {
		t.Fatalf("ping must contain only text: %s", raw)
	}
}

func TestPingRejectsUnexpectedReply(t *testing.T) {
	for _, text := range []string{"", "not pong", "{\"pong\":true}"} {
		f := newFakeJSON(t, ok, openAIReply("stop", text, ""))
		if err := NewOpenAI("", "model", f.srv.URL).Ping(context.Background()); err == nil {
			t.Fatalf("accepted reply %q", text)
		}
	}
}

func TestOpenAIPingUsesEffortOverride(t *testing.T) {
	f := newFakeJSON(t, ok, openAIReply("stop", "pong", ""))
	reader := NewOpenAI("", "model", f.srv.URL)
	reader.Effort = "none"
	if err := reader.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, present := f.body["reasoning_effort"]; present {
		t.Fatal("ignored configured effort opt-out")
	}
}
