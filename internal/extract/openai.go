package extract

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// OpenAIBaseURL is used unless MAHLZEIT_EXTRACT_BASE_URL names another
// OpenAI-compatible API, e.g. https://openrouter.ai/api/v1 or a local
// Ollama at http://ollama:11434/v1.
const OpenAIBaseURL = "https://api.openai.com/v1"

// OpenAI calls the chat completions endpoint of an OpenAI-compatible API.
// The model must read images, and PDFs for PDF receipts.
type OpenAI struct {
	client  *http.Client
	apiKey  string
	model   string
	baseURL string
}

// NewOpenAI creates a reader. The model is required; the key may be empty
// for local servers.
func NewOpenAI(apiKey, model, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = OpenAIBaseURL
	}
	return &OpenAI{client: http.DefaultClient, apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/")}
}

type openAIResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"message"`
	} `json:"choices"`
}

// Extract implements app.Extractor.
func (o *OpenAI) Extract(ctx context.Context, mime string, data []byte) (app.Extraction, error) {
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	file := map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL}}
	if mime == "application/pdf" {
		file = map[string]any{"type": "file", "file": map[string]any{"filename": "receipt.pdf", "file_data": dataURL}}
	}
	body := map[string]any{
		"model": o.model,
		"messages": []any{map[string]any{"role": "user", "content": []any{
			file,
			map[string]any{"type": "text", "text": prompt},
		}}},
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "receipt", "strict": true, "schema": schema},
		},
	}
	header := http.Header{}
	if o.apiKey != "" {
		header.Set("Authorization", "Bearer "+o.apiKey)
	}
	var resp openAIResponse
	if err := postJSON(ctx, o.client, o.baseURL+"/chat/completions", header, body, &resp); err != nil {
		return app.Extraction{}, err
	}
	if len(resp.Choices) == 0 {
		return app.Extraction{}, errDeclined
	}
	c := resp.Choices[0]
	if c.Message.Refusal != "" || c.FinishReason == "content_filter" {
		return app.Extraction{}, errDeclined
	}
	return parseAnswer(c.Message.Content)
}
