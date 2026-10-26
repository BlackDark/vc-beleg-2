package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// The database is the work queue. This signal only wakes an idle worker.
func (a *App) wakeExtraction() {
	select {
	case a.extractWake <- struct{}{}:
	default:
	}
}

// RunExtraction drains durable work one receipt at a time. Unfinished claims
// from a previous process are queued again; only one worker runs per App.
func (a *App) RunExtraction(ctx context.Context) {
	if a.Extractor == nil || !a.extractMu.TryLock() {
		return
	}
	defer a.extractMu.Unlock()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	// Retry initialization errors instead of leaving the worker unavailable.
	for ctx.Err() == nil {
		if _, err := a.DB.ExecContext(ctx, `UPDATE receipts SET extracting = 0 WHERE extraction = 'pending' AND extracting <> 0`); err == nil {
			break
		} else {
			a.Log.Error("extraction: recover", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
	for ctx.Err() == nil {
		job, err := a.claimExtraction(ctx)
		if err == nil {
			a.processExtraction(ctx, job)
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) && ctx.Err() == nil {
			a.Log.Error("extraction: claim", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-a.extractWake:
		case <-tick.C:
		}
	}
}

// RetryExtraction requeues a failed receipt. Repeating a queued or active
// retry is idempotent; successfully read and checked receipts are not retried.
func (a *App) RetryExtraction(ctx context.Context, u *User, id int64) (*Receipt, error) {
	if a.Extractor == nil {
		return nil, Invalid("extraction.off", "receipt reading is not configured")
	}
	res, err := a.DB.ExecContext(ctx, `UPDATE receipts SET extraction = 'pending', extracting = 0, extraction_json = '{}' WHERE id = ? AND employee_id = ? AND status = 'pending' AND extraction = 'failed'`, id, u.EmployeeID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		a.wakeExtraction()
		return a.GetReceipt(ctx, u, id)
	}
	r, err := a.GetReceipt(ctx, u, id)
	if err != nil {
		return nil, err
	}
	if r.Status == "pending" && (r.Extraction == "queued" || r.Extraction == "processing") {
		return r, nil
	}
	return nil, Invalid("extraction.not_failed", "only failed receipts to check can be read again")
}

// foundFields names what a reading found, for the import log.
func foundFields(ex Extraction) string {
	var found []string
	if ex.Date != "" {
		found = append(found, "date")
	}
	if ex.Total > 0 {
		found = append(found, "total")
	}
	if ex.Merchant != "" {
		found = append(found, "merchant")
	}
	if len(found) == 0 {
		return "nothing found"
	}
	return "found: " + strings.Join(found, ", ")
}
