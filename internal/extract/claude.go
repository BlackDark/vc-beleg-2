// Package extract reads date, total and merchant from receipt files with
// Claude. Results are suggestions; the employee checks every receipt.
package extract

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/money"
)

// DefaultModel is used unless MAHLZEIT_EXTRACT_MODEL names another.
const DefaultModel = "claude-opus-5-5"

// Claude calls the Messages API.
type Claude struct {
	client   anthropic.Client
	model    string
	fallback bool
}

// New creates a reader; the API key comes from the caller's configuration.
// With fallback, a refused request is answered by the fallback model the API
// picks for the refusal category instead of failing.
func New(apiKey, model string, fallback bool, opts ...option.RequestOption) *Claude {
	if model == "" {
		model = DefaultModel
	}
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(4)}, opts...)
	return &Claude{client: anthropic.NewClient(opts...), model: model, fallback: fallback}
}

const prompt = `This is a German shop or restaurant receipt (Kassenbon) photographed or scanned by its owner.
Read the purchase date, the total amount paid (Gesamtbetrag / Summe / zu zahlen, in EUR, including VAT) and the merchant's name.
Use null for a value you cannot read. List in "uncertain" every field you had to guess (blurry, cut off, several candidates).`

var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"date":      map[string]any{"type": []string{"string", "null"}, "description": "purchase date as YYYY-MM-DD"},
		"total":     map[string]any{"type": []string{"number", "null"}, "description": "total in EUR, e.g. 7.89"},
		"merchant":  map[string]any{"type": []string{"string", "null"}, "description": "shop name, e.g. REWE"},
		"uncertain": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"date", "total", "merchant"}}},
	},
	"required":             []string{"date", "total", "merchant", "uncertain"},
	"additionalProperties": false,
}

type answer struct {
	Date      *string  `json:"date"`
	Total     *float64 `json:"total"`
	Merchant  *string  `json:"merchant"`
	Uncertain []string `json:"uncertain"`
}

// Extract implements app.Extractor.
func (c *Claude) Extract(ctx context.Context, mime string, data []byte) (app.Extraction, error) {
	b64 := base64.StdEncoding.EncodeToString(data)
	var file anthropic.BetaContentBlockParamUnion
	if mime == "application/pdf" {
		file = anthropic.NewBetaDocumentBlock(anthropic.BetaBase64PDFSourceParam{Data: b64})
	} else {
		file = anthropic.NewBetaImageBlock(anthropic.BetaBase64ImageSourceParam{Data: b64, MediaType: anthropic.BetaBase64ImageSourceMediaType(mime)})
	}
	params := anthropic.BetaMessageNewParams{
		Model:     c.model,
		MaxTokens: 4000,
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortLow,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: schema},
		},
		Messages: []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(file, anthropic.NewBetaTextBlock(prompt))},
	}
	if c.fallback {
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamOfDefault()
	}
	resp, err := c.client.Beta.Messages.New(ctx, params)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return app.Extraction{}, fmt.Errorf("claude api: status %d", apiErr.StatusCode)
		}
		return app.Extraction{}, err
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return app.Extraction{}, errors.New("claude declined the request")
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	var a answer
	if err := json.Unmarshal([]byte(text.String()), &a); err != nil {
		return app.Extraction{}, fmt.Errorf("unexpected answer: %w", err)
	}
	return convert(a), nil
}

// convert validates the answer; implausible values are dropped and marked
// uncertain instead of being saved.
func convert(a answer) app.Extraction {
	ex := app.Extraction{Uncertain: a.Uncertain}
	if a.Date != nil {
		if _, err := time.Parse(time.DateOnly, *a.Date); err == nil {
			ex.Date = *a.Date
		} else {
			ex.Uncertain = append(ex.Uncertain, "date")
		}
	}
	if a.Total != nil && *a.Total > 0 && *a.Total < 1000 {
		ex.Total = money.Cents(math.Round(*a.Total * 100))
	} else if a.Total != nil {
		ex.Uncertain = append(ex.Uncertain, "total")
	}
	if a.Merchant != nil {
		ex.Merchant = strings.TrimSpace(*a.Merchant)
		if len(ex.Merchant) > 200 {
			ex.Merchant = ex.Merchant[:200]
		}
	}
	return ex
}
