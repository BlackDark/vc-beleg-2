package extract

import (
	"context"
	"testing"
)

func TestOpenAIDefaultLowEffort(t *testing.T) {
	f := newFakeJSON(t, ok, openAIReply("stop", `{"date":null,"total":null,"merchant":null,"uncertain":[]}`, ""))
	if _, err := NewOpenAI("", "model", f.srv.URL).Extract(context.Background(), "image/png", []byte("png")); err != nil {
		t.Fatal(err)
	}
	if f.body["reasoning_effort"] != "low" {
		t.Fatalf("default reasoning effort = %v, want low", f.body["reasoning_effort"])
	}
}

func TestOpenAIEffortOverride(t *testing.T) {
	for _, effort := range []string{"medium", "high", "none"} {
		t.Run(effort, func(t *testing.T) {
			f := newFakeJSON(t, ok, openAIReply("stop", `{"date":null,"total":null,"merchant":null,"uncertain":[]}`, ""))
			reader := NewOpenAI("", "model", f.srv.URL)
			reader.Effort = effort
			if _, err := reader.Extract(context.Background(), "image/png", []byte("png")); err != nil {
				t.Fatal(err)
			}
			got, present := f.body["reasoning_effort"]
			if effort == "none" {
				if present {
					t.Fatalf("unsupported effort field sent: %v", got)
				}
				return
			}
			if got != effort {
				t.Fatalf("effort=%v, want %s", got, effort)
			}
		})
	}
}
