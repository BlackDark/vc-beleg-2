package main

import (
	"os"
	"strings"
	"testing"
)

func TestExtractConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    ExtractConfig
		wantErr string
	}{
		{"claude without key is off", nil, ExtractConfig{Provider: "claude", Fallback: true}, ""},
		{"claude", map[string]string{"ANTHROPIC_API_KEY": "a"}, ExtractConfig{Provider: "claude", Enabled: true, APIKey: "a", Fallback: true}, ""},
		{"gemini", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "gemini", "GEMINI_API_KEY": "g"}, ExtractConfig{Provider: "gemini", Enabled: true, APIKey: "g", Fallback: true}, ""},
		{"gemini needs a key", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "gemini"}, ExtractConfig{}, "GEMINI_API_KEY is required"},
		{"openrouter", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "openai", "OPENAI_API_KEY": "o", "MAHLZEIT_EXTRACT_MODEL": "google/gemini-flash", "MAHLZEIT_EXTRACT_BASE_URL": "https://openrouter.ai/api/v1"},
			ExtractConfig{Provider: "openai", Enabled: true, APIKey: "o", Model: "google/gemini-flash", BaseURL: "https://openrouter.ai/api/v1", Fallback: true}, ""},
		{"ollama without key", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "openai", "MAHLZEIT_EXTRACT_MODEL": "qwen2.5vl", "MAHLZEIT_EXTRACT_BASE_URL": "http://ollama:11434/v1"},
			ExtractConfig{Provider: "openai", Enabled: true, Model: "qwen2.5vl", BaseURL: "http://ollama:11434/v1", Fallback: true}, ""},
		{"openai needs a model", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "openai"}, ExtractConfig{}, "MAHLZEIT_EXTRACT_MODEL is required"},
		{"unknown provider", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "mistral"}, ExtractConfig{}, "must be claude, gemini or openai"},
		{"bad base URL", map[string]string{"MAHLZEIT_EXTRACT_PROVIDER": "openai", "MAHLZEIT_EXTRACT_MODEL": "m", "MAHLZEIT_EXTRACT_BASE_URL": "openrouter.ai"}, ExtractConfig{}, "MAHLZEIT_EXTRACT_BASE_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"MAHLZEIT_EXTRACT_PROVIDER", "MAHLZEIT_EXTRACT_MODEL", "MAHLZEIT_EXTRACT_BASE_URL", "MAHLZEIT_EXTRACT_FALLBACK", "ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY_FILE", "GEMINI_API_KEY", "GEMINI_API_KEY_FILE", "OPENAI_API_KEY", "OPENAI_API_KEY_FILE"} {
				t.Setenv(k, "") // restores the variable after the test
				os.Unsetenv(k)
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, err := extractConfig()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %+v, %v\nwant %+v", got, err, tt.want)
			}
		})
	}
}
