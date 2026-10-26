package extract

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const pingPrompt = "Connection test: reply with exactly pong."

func checkPong(text string) error {
	if strings.EqualFold(strings.TrimSpace(text), "pong") {
		return nil
	}
	return errors.New("model did not reply with pong")
}

// Ping checks text access to the configured model; it sends no receipt data.
func (o *OpenAI) Ping(ctx context.Context) error {
	resp, err := o.request(ctx, map[string]any{"messages": []any{map[string]any{"role": "user", "content": pingPrompt}}})
	if err != nil {
		return err
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Refusal != "" {
		return errDeclined
	}
	return checkPong(resp.Choices[0].Message.Content)
}

// Ping checks text access to the configured model; it sends no receipt data.
func (g *Gemini) Ping(ctx context.Context) error {
	resp, err := g.request(ctx, map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": pingPrompt}}}}})
	if err != nil {
		return err
	}
	text, err := geminiText(resp)
	if err != nil {
		return err
	}
	return checkPong(text)
}

// Ping checks text access to the configured model; it sends no receipt data.
func (c *Claude) Ping(ctx context.Context) error {
	params := c.params()
	params.MaxTokens = 64
	params.Messages = []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(pingPrompt))}
	text, err := c.request(ctx, params)
	if err != nil {
		return err
	}
	return checkPong(text)
}
