package app

import "time"

// ReceiptReadingTimeout bounds provider retries and reading one receipt.
const ReceiptReadingTimeout = 3 * time.Minute

// ReaderInfo describes configured receipt reading without credentials.
type ReaderInfo struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Endpoint       string `json:"endpoint"`
	Effort         string `json:"effort" doc:"low, medium, high, none, or provider_default"`
	Fallback       bool   `json:"fallback"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Sequential     bool   `json:"sequential"`
}
