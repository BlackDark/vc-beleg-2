package extract

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// ClaudeModel is used unless MAHLZEIT_EXTRACT_MODEL names another.
const ClaudeModel = "claude-opus-5-5"

// Claude calls the Messages API.
type Claude struct {
	client   anthropic.Client
	model    string
	fallback bool
}

// NewClaude creates a reader; the API key comes from the caller's
// configuration. With fallback, a refused request is answered by the
// fallback model the API picks for the refusal category instead of failing.
func NewClaude(apiKey, model string, fallback bool, opts ...option.RequestOption) *Claude {
	if model == "" {
		model = ClaudeModel
	}
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(4)}, opts...)
	return &Claude{client: anthropic.NewClient(opts...), model: model, fallback: fallback}
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
	params := c.params()
	params.OutputConfig.Format = anthropic.BetaJSONOutputFormatParam{Schema: schema}
	params.Messages = []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(file, anthropic.NewBetaTextBlock(prompt))}
	text, err := c.request(ctx, params)
	if err != nil {
		return app.Extraction{}, err
	}
	return parseAnswer(text)
}

func (c *Claude) params() anthropic.BetaMessageNewParams {
	params := anthropic.BetaMessageNewParams{Model: c.model, MaxTokens: 4000, OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortLow}}
	if c.fallback {
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamOfDefault()
	}
	return params
}

func (c *Claude) request(ctx context.Context, params anthropic.BetaMessageNewParams) (string, error) {
	resp, err := c.client.Beta.Messages.New(ctx, params)
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return "", fmt.Errorf("claude api: status %d", apiErr.StatusCode)
		}
		return "", err
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return "", errors.New("claude declined the request")
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	return text.String(), nil
}
