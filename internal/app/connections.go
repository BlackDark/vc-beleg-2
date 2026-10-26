package app

import (
	"context"
	"errors"
	"time"
)

// ConnectionResult reports a bounded connection check without remote error text.
type ConnectionResult struct {
	OK         bool   `json:"ok"`
	DurationMS int64  `json:"durationMs"`
	ErrorCode  string `json:"errorCode" doc:"empty on success; failed or timeout otherwise"`
}

// CheckConnection runs an explicitly requested check; it never imports receipts.
func (a *App) CheckConnection(ctx context.Context, service string) (*ConnectionResult, error) {
	switch service {
	case "reader", "mail", "folder":
	default:
		return nil, Invalid("service.unknown", "unknown service")
	}
	check := a.ConnectionChecks[service]
	if check == nil {
		return nil, Invalid("service.disabled", "this service is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	err := check(ctx)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	result := &ConnectionResult{OK: err == nil, DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		result.ErrorCode = "failed"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			result.ErrorCode = "timeout"
		}
	}
	// Remote errors can echo credentials or user content. Log only the outcome.
	a.Log.Info("connection checked", "service", service, "ok", result.OK, "duration_ms", result.DurationMS, "error_code", result.ErrorCode)
	return result, nil
}
