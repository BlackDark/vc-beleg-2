// Package extract reads date, total and merchant from receipt files with an
// LLM: Claude, Gemini or any OpenAI-compatible API. Results are suggestions;
// the employee checks every receipt.
package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/money"
)

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

// parseAnswer reads the model's JSON. Models without structured output may
// wrap it in a Markdown code fence.
func parseAnswer(text string) (app.Extraction, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	var a answer
	if err := json.Unmarshal([]byte(text), &a); err != nil {
		return app.Extraction{}, fmt.Errorf("unexpected answer: %w", err)
	}
	return convert(a), nil
}

// errDeclined marks a request the model refused to answer.
var errDeclined = errors.New("the model declined the request")

// retryWaits are the pauses before retrying a rate-limited or failed request.
var retryWaits = []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second}

// postJSON sends body to url and decodes the JSON response into out. Rate
// limits (429) and server errors (5xx) are retried; error messages carry the
// status only, never the response body, which may echo receipt content.
func postJSON(ctx context.Context, client *http.Client, url string, header http.Header, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header = header.Clone()
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if retry && attempt < len(retryWaits) {
			resp.Body.Close()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryWaits[attempt]):
			}
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			return fmt.Errorf("api: status %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
}
