package app

import (
	"context"
	"errors"
	"time"
)

// LogEntry is one line of the import log (Import-Protokoll).
type LogEntry struct {
	ID        int64  `json:"id"`
	At        string `json:"at"`
	Source    string `json:"source" enum:"upload,mail,folder,reader"`
	Filename  string `json:"filename"`
	Outcome   string `json:"outcome" enum:"imported,duplicate,rejected,read,read_failed"`
	Message   string `json:"message" doc:"reason, or reader and duration"`
	ReceiptID int64  `json:"receiptId" doc:"0 when no receipt was created"`
}

// logRetention is how long import log entries are kept.
const logRetention = 90 * 24 * time.Hour

// Record adds an entry to the import log of the account's employee. The log
// is informational: a failure to write it is logged, never returned.
func (a *App) Record(ctx context.Context, u *User, e LogEntry) {
	if u == nil {
		return
	}
	now := a.now().UTC()
	_, err := a.DB.ExecContext(ctx, `INSERT INTO import_log(employee_id, at, source, filename, outcome, message, receipt_id) VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 0))`,
		u.EmployeeID, now.Format(time.RFC3339), e.Source, e.Filename, e.Outcome, e.Message, e.ReceiptID)
	if err == nil {
		_, err = a.DB.ExecContext(ctx, `DELETE FROM import_log WHERE employee_id = ? AND at < ?`, u.EmployeeID, now.Add(-logRetention).Format(time.RFC3339))
	}
	if err != nil {
		a.Log.Error("import log: write", "err", err)
	}
}

// ImportLog returns the latest entries, newest first.
func (a *App) ImportLog(ctx context.Context, u *User, limit int) ([]LogEntry, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT id, at, source, filename, outcome, message, COALESCE(receipt_id, 0) FROM import_log WHERE employee_id = ? ORDER BY at DESC, id DESC LIMIT ?`, u.EmployeeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Source, &e.Filename, &e.Outcome, &e.Message, &e.ReceiptID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// recordImport logs the outcome of Import: a receipt, or why the file was
// not accepted.
func (a *App) recordImport(ctx context.Context, u *User, source, filename string, r *Receipt, err error) {
	var invalid *Error
	switch {
	case err == nil:
		a.Record(ctx, u, LogEntry{Source: source, Filename: filename, Outcome: "imported", ReceiptID: r.ID})
	case errors.As(err, &invalid):
		a.Record(ctx, u, LogEntry{Source: source, Filename: filename, Outcome: "rejected", Message: invalid.Message})
	}
}
