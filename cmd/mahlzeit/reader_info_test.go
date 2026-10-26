package main

import (
	"strings"
	"testing"
)

func TestReaderInfo(t *testing.T) {
	for _, tc := range []struct{ provider, model, effort string }{
		{"claude", "claude-opus-5-5", "low"},
		{"gemini", "gemini-flash-latest", "provider_default"},
		{"openai", "custom-model", "low"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			x := ExtractConfig{Provider: tc.provider, Enabled: true, Fallback: true}
			if tc.provider == "openai" {
				x.Model = "custom-model"
			}
			info := x.Info()
			if info.Model != tc.model || info.Effort != tc.effort || !info.Sequential || info.TimeoutSeconds != 180 || !info.Enabled {
				t.Fatalf("reader info: %+v", info)
			}
			if info.Fallback != (tc.provider == "claude") {
				t.Fatalf("fallback: %+v", info)
			}
			x.BaseURL = "https://name:password@example.com/v1?api_key=secret#fragment"
			info = x.Info()
			if info.Endpoint != "https://example.com/v1" || strings.Contains(info.Endpoint, "secret") {
				t.Fatalf("unsafe endpoint: %s", info.Endpoint)
			}
		})
	}
}

func TestExtractEffortConfiguration(t *testing.T) {
	t.Setenv("MAHLZEIT_EXTRACT_PROVIDER", "openai")
	t.Setenv("MAHLZEIT_EXTRACT_MODEL", "custom-model")
	t.Setenv("MAHLZEIT_EXTRACT_BASE_URL", "")
	for _, effort := range []string{"", "low", "medium", "high", "none", "invalid"} {
		t.Run(effort, func(t *testing.T) {
			t.Setenv("MAHLZEIT_EXTRACT_EFFORT", effort)
			x, err := extractConfig()
			if effort == "invalid" {
				if err == nil {
					t.Fatal("accepted invalid effort")
				}
				return
			}
			want := effort
			if want == "" {
				want = "low"
			}
			if err != nil || x.Effort != want {
				t.Fatalf("effort=%s error=%v", x.Effort, err)
			}
		})
	}
}
