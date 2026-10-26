package main

import (
	"net/url"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/extract"
)

func (x ExtractConfig) Info() app.ReaderInfo {
	info := app.ReaderInfo{Enabled: x.Enabled, Provider: x.Provider, Model: x.Model, Endpoint: x.BaseURL, Effort: "provider_default", TimeoutSeconds: int(app.ReceiptReadingTimeout / time.Second), Sequential: true}
	switch x.Provider {
	case "claude":
		if info.Model == "" {
			info.Model = extract.ClaudeModel
		}
		if info.Endpoint == "" {
			info.Endpoint = "https://api.anthropic.com"
		}
		info.Effort, info.Fallback = "low", x.Fallback
	case "gemini":
		if info.Model == "" {
			info.Model = extract.GeminiModel
		}
		if info.Endpoint == "" {
			info.Endpoint = extract.GeminiBaseURL
		}
	case "openai":
		if info.Endpoint == "" {
			info.Endpoint = extract.OpenAIBaseURL
		}
		info.Effort = x.Effort
		if info.Effort == "" {
			info.Effort = "low"
		}
	}
	if u, err := url.Parse(info.Endpoint); err == nil {
		u.User, u.RawQuery, u.Fragment = nil, "", ""
		info.Endpoint = u.String()
	} else {
		info.Endpoint = ""
	}
	return info
}
