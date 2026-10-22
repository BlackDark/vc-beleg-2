// Package app holds the domain services: persistence, ownership checks,
// audit events, receipts and months. HTTP handlers live in internal/api.
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/export"
	"github.com/blackdark/vc-beleg-2/internal/rates"
)

// Error is a domain error with a stable code the UI translates.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Params  map[string]string `json:"params,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Common errors. Missing and foreign objects look the same.
var (
	ErrNotFound     = &Error{Status: 404, Code: "not_found", Message: "not found"}
	ErrUnauthorized = &Error{Status: 401, Code: "unauthorized", Message: "login required"}
	ErrMonthClosed  = &Error{Status: 409, Code: "month.closed", Message: "the month is closed; reopen it first"}
)

// Invalid returns a validation error.
func Invalid(code, msg string, params ...string) *Error {
	e := &Error{Status: 422, Code: code, Message: msg}
	if len(params) > 0 {
		e.Params = map[string]string{}
		for i := 0; i+1 < len(params); i += 2 {
			e.Params[params[i]] = params[i+1]
		}
	}
	return e
}

// Config configures the services.
type Config struct {
	DataDir        string
	Typst          string
	FontDir        string
	MaxUploadBytes int64
	MaxPixels      int64
	MaxPDFPages    int
}

// Extractor reads date, total and merchant from a receipt file.
type Extractor interface {
	Extract(ctx context.Context, mime string, data []byte) (Extraction, error)
}

// App bundles the database, rate table, file storage and Typst.
type App struct {
	DB        *sql.DB
	Cfg       Config
	Log       *slog.Logger
	Files     *Files
	Typst     *export.Typst
	Rates     rates.Table
	Extractor Extractor // nil: extraction is off

	extractQueue chan int64
	mail         mailState
	now          func() time.Time
}

// New creates the services.
func New(db *sql.DB, cfg Config, log *slog.Logger, ex Extractor) (*App, error) {
	files, err := NewFiles(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	tab, err := rates.Load()
	if err != nil {
		return nil, err
	}
	return &App{DB: db, Cfg: cfg, Log: log, Files: files, Rates: tab, Extractor: ex, now: time.Now,
		Typst:        &export.Typst{Bin: cfg.Typst, FontDir: cfg.FontDir},
		extractQueue: make(chan int64, 256)}, nil
}

func (a *App) stamp() string { return a.now().UTC().Format(time.RFC3339) }

// tx runs fn in a transaction.
func (a *App) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// audit appends an audit event inside the transaction committing the change.
func (a *App) audit(ctx context.Context, tx *sql.Tx, userID int64, entity string, id int64, action, reason string, data any) error {
	b := []byte("{}")
	if data != nil {
		var err error
		if b, err = json.Marshal(data); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_log(user_id, entity, entity_id, action, reason, data_json, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, entity, id, action, reason, string(b), a.stamp())
	return err
}

// notFound maps sql.ErrNoRows to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
