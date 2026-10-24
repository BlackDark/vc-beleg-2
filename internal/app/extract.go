package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (a *App) enqueueExtraction(id int64) {
	select {
	case a.extractQueue <- id:
	default:
		// Full queue: the receipt stays pending and is picked up on restart.
		a.Log.Warn("extraction queue full", "receipt", id)
	}
}

// RunExtraction reads queued receipts one at a time until ctx ends. Receipts
// left pending by a restart are queued first.
func (a *App) RunExtraction(ctx context.Context) {
	if a.Extractor == nil {
		return
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT id FROM receipts WHERE extraction = 'pending' ORDER BY id`)
	if err == nil {
		var ids []int64
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		for _, id := range ids {
			a.enqueueExtraction(id)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-a.extractQueue:
			a.extractOne(ctx, id)
		}
	}
}

// RetryExtraction queues a receipt again, e.g. after a failure.
func (a *App) RetryExtraction(ctx context.Context, u *User, id int64) (*Receipt, error) {
	if a.Extractor == nil {
		return nil, Invalid("extraction.off", "receipt reading is not configured")
	}
	res, err := a.DB.ExecContext(ctx, `UPDATE receipts SET extraction = 'pending' WHERE id = ? AND employee_id = ? AND status = 'pending'`, id, u.EmployeeID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, Invalid("extraction.not_pending", "only receipts to check can be read again")
	}
	a.enqueueExtraction(id)
	return a.GetReceipt(ctx, u, id)
}

func (a *App) extractOne(ctx context.Context, id int64) {
	var key, mime, filename string
	var employee int64
	if err := a.DB.QueryRowContext(ctx, `SELECT storage_key, mime, filename, employee_id FROM receipts WHERE id = ? AND extraction = 'pending'`, id).Scan(&key, &mime, &filename, &employee); err != nil {
		return
	}
	owner := &User{EmployeeID: employee}
	data, err := a.Files.Read(key)
	if err != nil {
		a.Log.Error("extraction: read file", "receipt", id, "err", err)
		return
	}
	a.Log.Info("extraction started", "receipt", id, "reader", a.Cfg.ReaderLabel, "mime", mime, "bytes", len(data))
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	ex, err := a.Extractor.Extract(cctx, mime, data)
	cancel()
	took := time.Since(start).Round(100 * time.Millisecond)
	state := "done"
	if err != nil {
		if ctx.Err() != nil {
			return // shutting down; stays pending
		}
		a.Log.Warn("extraction failed", "receipt", id, "reader", a.Cfg.ReaderLabel, "took", took.String(), "err", err)
		a.Record(ctx, owner, LogEntry{Source: "reader", Filename: filename, Outcome: "read_failed", ReceiptID: id,
			Message: a.Cfg.ReaderLabel + " · " + took.String() + " · " + err.Error()})
		state, ex = "failed", Extraction{Error: err.Error()}
	} else {
		// Which fields were found, never their values.
		a.Log.Info("extraction done", "receipt", id, "took", took.String(),
			"date", ex.Date != "", "total", ex.Total > 0, "merchant", ex.Merchant != "", "uncertain", ex.Uncertain)
		a.Record(ctx, owner, LogEntry{Source: "reader", Filename: filename, Outcome: "read", ReceiptID: id,
			Message: a.Cfg.ReaderLabel + " · " + took.String() + " · " + foundFields(ex)})
	}
	b, _ := json.Marshal(ex)
	// Fill only what the employee has not entered meanwhile; checked
	// receipts keep their values.
	_, err = a.DB.ExecContext(ctx, `UPDATE receipts SET extraction = ?, extraction_json = ?,
		date = CASE WHEN status = 'pending' AND date IS NULL AND ? <> '' THEN ? ELSE date END,
		total = CASE WHEN status = 'pending' AND total IS NULL AND ? > 0 THEN ? ELSE total END,
		merchant = CASE WHEN status = 'pending' AND merchant = '' THEN ? ELSE merchant END
		WHERE id = ?`, state, string(b), ex.Date, ex.Date, ex.Total, ex.Total, ex.Merchant, id)
	if err != nil {
		a.Log.Error("extraction: save", "receipt", id, "err", err)
	}
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
