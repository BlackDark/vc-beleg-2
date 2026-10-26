package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type extractionJob struct {
	id, employee        int64
	key, mime, filename string
}

func (a *App) claimExtraction(ctx context.Context) (extractionJob, error) {
	var job extractionJob
	err := a.DB.QueryRowContext(ctx, `UPDATE receipts SET extracting = 1
 WHERE id = (SELECT id FROM receipts WHERE extraction = 'pending' AND extracting = 0 ORDER BY id LIMIT 1)
 AND extraction = 'pending' AND extracting = 0
 RETURNING id, employee_id, storage_key, mime, filename`).Scan(&job.id, &job.employee, &job.key, &job.mime, &job.filename)
	return job, err
}

func (a *App) processExtraction(ctx context.Context, job extractionJob) {
	start := time.Now()
	data, err := a.Files.Read(job.key)
	var ex Extraction
	if err != nil {
		a.Log.Error("extraction: read file", "receipt", job.id, "err", err)
		err = errors.New("the receipt file could not be read")
	} else {
		a.Log.Info("extraction started", "receipt", job.id, "reader", a.Cfg.ReaderLabel, "mime", job.mime, "bytes", len(data))
		readCtx, cancel := context.WithTimeout(ctx, ReceiptReadingTimeout)
		ex, err = a.Extractor.Extract(readCtx, job.mime, data)
		cancel()
	}
	took := time.Since(start).Round(100 * time.Millisecond)
	for ctx.Err() == nil {
		saved, saveErr := a.finishExtraction(ctx, job, ex, err, took)
		if saveErr == nil {
			if saved {
				a.Log.Info("extraction completed", "receipt", job.id, "reader", a.Cfg.ReaderLabel, "took", took.String(), "failed", err != nil)
			}
			return
		}
		a.Log.Error("extraction: save", "receipt", job.id, "err", saveErr)
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	// A canceled request remains recoverable, including when this App is reused.
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, releaseErr := a.DB.ExecContext(releaseCtx, `UPDATE receipts SET extracting = 0 WHERE id = ? AND storage_key = ? AND extraction = 'pending' AND extracting = 1`, job.id, job.key); releaseErr != nil {
		a.Log.Error("extraction: release", "receipt", job.id, "err", releaseErr)
	}
}

func (a *App) finishExtraction(ctx context.Context, job extractionJob, ex Extraction, readErr error, took time.Duration) (bool, error) {
	state, outcome, message := "done", "read", foundFields(ex)
	if readErr != nil {
		state, outcome, message, ex = "failed", "read_failed", readErr.Error(), Extraction{Error: readErr.Error()}
	}
	b, _ := json.Marshal(ex)
	saved := false
	err := a.tx(ctx, func(tx *sql.Tx) error {
		// The original key guards against deleted IDs being reused during a read.
		// Employee edits win, and checking a receipt never depends on the reader.
		res, err := tx.ExecContext(ctx, `UPDATE receipts SET extraction = ?, extracting = 0, extraction_json = ?,
   date = CASE WHEN status = 'pending' AND date IS NULL AND ? <> '' THEN ? ELSE date END,
   total = CASE WHEN status = 'pending' AND total IS NULL AND ? > 0 THEN ? ELSE total END,
   merchant = CASE WHEN status = 'pending' AND merchant = '' THEN ? ELSE merchant END
   WHERE id = ? AND storage_key = ? AND extraction = 'pending' AND extracting = 1`, state, string(b), ex.Date, ex.Date, ex.Total, ex.Total, ex.Merchant, job.id, job.key)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil || n == 0 {
			return err
		}
		saved = true
		return a.record(ctx, tx, &User{EmployeeID: job.employee}, LogEntry{Source: "reader", Filename: job.filename, Outcome: outcome, ReceiptID: job.id,
			Message: a.Cfg.ReaderLabel + " · " + took.String() + " · " + message})
	})
	return saved, err
}
