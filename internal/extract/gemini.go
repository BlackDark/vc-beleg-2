package extract

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// GeminiModel is Google's alias for its current Flash model; it is used
// unless MAHLZEIT_EXTRACT_MODEL names another.
const GeminiModel = "gemini-flash-latest"

// GeminiBaseURL is the Gemini API (Google AI Studio keys).
const GeminiBaseURL = "https://generativelanguage.googleapis.com"

// Gemini calls the generateContent method of the Gemini API.
type Gemini struct {
	client  *http.Client
	apiKey  string
	model   string
	baseURL string
}

// NewGemini creates a reader; empty model and base URL take the defaults.
func NewGemini(apiKey, model, baseURL string) *Gemini {
	if model == "" {
		model = GeminiModel
	}
	if baseURL == "" {
		baseURL = GeminiBaseURL
	}
	return &Gemini{client: http.DefaultClient, apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/")}
}

// geminiSchema is the answer schema in the API's OpenAPI subset.
var geminiSchema = map[string]any{
	"type": "OBJECT",
	"properties": map[string]any{
		"date":      map[string]any{"type": "STRING", "nullable": true, "description": "purchase date as YYYY-MM-DD"},
		"total":     map[string]any{"type": "NUMBER", "nullable": true, "description": "total in EUR, e.g. 7.89"},
		"merchant":  map[string]any{"type": "STRING", "nullable": true, "description": "shop name, e.g. REWE"},
		"uncertain": map[string]any{"type": "ARRAY", "items": map[string]any{"type": "STRING", "enum": []string{"date", "total", "merchant"}}},
	},
	"required": []string{"date", "total", "merchant", "uncertain"},
}

type geminiResponse struct {
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	Candidates []struct {
		FinishReason string `json:"finishReason"`
		Content      struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// Extract implements app.Extractor.
func (g *Gemini) Extract(ctx context.Context, mime string, data []byte) (app.Extraction, error) {
	body := map[string]any{
		"contents": []any{map[string]any{"role": "user", "parts": []any{
			map[string]any{"inlineData": map[string]any{"mimeType": mime, "data": base64.StdEncoding.EncodeToString(data)}},
			map[string]any{"text": prompt},
		}}},
		"generationConfig": map[string]any{"responseMimeType": "application/json", "responseSchema": geminiSchema},
	}
	resp, err := g.request(ctx, body)
	if err != nil {
		return app.Extraction{}, err
	}
	text, err := geminiText(resp)
	if err != nil {
		return app.Extraction{}, err
	}
	return parseAnswer(text)
}

func geminiText(resp geminiResponse) (string, error) {
	if resp.PromptFeedback.BlockReason != "" || len(resp.Candidates) == 0 {
		return "", errDeclined
	}
	c := resp.Candidates[0]
	switch c.FinishReason {
	case "SAFETY", "PROHIBITED_CONTENT", "BLOCKLIST", "SPII", "RECITATION":
		return "", errDeclined
	}
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	return text.String(), nil
}

func (g *Gemini) request(ctx context.Context, body map[string]any) (geminiResponse, error) {
	header := http.Header{"X-Goog-Api-Key": {g.apiKey}}
	var resp geminiResponse
	endpoint := g.baseURL + "/v1beta/models/" + url.PathEscape(g.model) + ":generateContent"
	if err := postJSON(ctx, g.client, endpoint, header, body, &resp); err != nil {
		return resp, err
	}
	return resp, nil
}
