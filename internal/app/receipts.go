package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // decode configs for pixel limits
	_ "image/png"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/money"
)

// Extraction is what the reader found on a receipt. Uncertain lists the
// fields it was not sure about.
type Extraction struct {
	Date      string      `json:"date,omitempty"`
	Total     money.Cents `json:"total,omitempty"`
	Merchant  string      `json:"merchant,omitempty"`
	Uncertain []string    `json:"uncertain,omitempty" doc:"date, total, merchant"`
	Error     string      `json:"error,omitempty"`
}

// Receipt is an imported receipt (Beleg).
type Receipt struct {
	ID         int64       `json:"id"`
	Status     string      `json:"status" enum:"pending,checked"`
	Source     string      `json:"source" enum:"upload,mail,folder"`
	Number     string      `json:"number"`
	Filename   string      `json:"filename"`
	Mime       string      `json:"mime"`
	Pages      int         `json:"pages"`
	Date       string      `json:"date"`
	Total      money.Cents `json:"total"`
	Merchant   string      `json:"merchant"`
	Note       string      `json:"note"`
	MailFrom   string      `json:"mailFrom"`
	Extraction string      `json:"extraction" enum:"off,pending,done,failed"`
	Extracted  Extraction  `json:"extracted"`
	CreatedAt  string      `json:"createdAt"`
	CheckedAt  string      `json:"checkedAt"`
	Warnings   []string    `json:"warnings" doc:"duplicate, date_missing, total_missing, month_closed, future_date"`
}

// MailMeta identifies a receipt imported from a mail.
type MailMeta struct {
	MessageID string
	From      string
}

// detect validates a file by content and returns its MIME type and extension.
// DetectType returns the receipt type of a file by its content, as the
// import sees it.
func DetectType(b []byte) (mime string, ok bool) {
	mime, _, ok = detect(b)
	return mime, ok
}

func detect(b []byte) (string, string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", ".jpg", true
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", ".png", true
	case bytes.HasPrefix(b, []byte("%PDF-")):
		return "application/pdf", ".pdf", true
	}
	return "", "", false
}

// Import stores an original receipt file as pending and queues extraction.
func (a *App) Import(ctx context.Context, u *User, source, filename string, data []byte, note string, mail *MailMeta) (*Receipt, error) {
	r, err := a.importFile(ctx, u, source, filename, data, note, mail)
	a.recordImport(ctx, u, source, filename, r, err)
	return r, err
}

func (a *App) importFile(ctx context.Context, u *User, source, filename string, data []byte, note string, mail *MailMeta) (*Receipt, error) {
	if int64(len(data)) > a.Cfg.MaxUploadBytes {
		return nil, Invalid("upload.too_large", "file exceeds the size limit", "limitMb", fmt.Sprint(a.Cfg.MaxUploadBytes>>20))
	}
	mime, ext, ok := detect(data)
	if !ok {
		return nil, Invalid("upload.unsupported", "only JPEG, PNG and PDF files are supported")
	}
	pages := 1
	if mime == "application/pdf" {
		if bytes.Contains(data, []byte("/Encrypt")) {
			return nil, Invalid("upload.pdf_encrypted", "encrypted PDFs are not supported")
		}
		n, err := a.Typst.PDFInfo(ctx, data)
		if err != nil {
			return nil, Invalid("upload.pdf_invalid", "the PDF could not be read")
		}
		if n > a.Cfg.MaxPDFPages {
			return nil, Invalid("upload.too_many_pages", "the PDF has too many pages", "limit", fmt.Sprint(a.Cfg.MaxPDFPages))
		}
		pages = n
	} else {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, Invalid("upload.image_invalid", "the image could not be read")
		}
		if int64(cfg.Width)*int64(cfg.Height) > a.Cfg.MaxPixels {
			return nil, Invalid("upload.too_many_pixels", "the image resolution is too large")
		}
	}
	// Rendering the preview validates the whole file the way the report will
	// read it, and drops EXIF data such as GPS positions.
	png, err := a.Typst.Preview(ctx, data, ext, 600)
	if err != nil {
		return nil, Invalid("upload.unreadable", "the file could not be read; please upload a valid JPEG, PNG or PDF")
	}
	key, sum, size, err := a.Files.Put("files", ext, bytes.NewReader(data), a.Cfg.MaxUploadBytes)
	if err != nil {
		return nil, err
	}
	previewKey, _, _, err := a.Files.Put("files", ".png", bytes.NewReader(png), 1<<26)
	if err != nil {
		a.Files.Remove(key)
		return nil, err
	}
	extraction := "off"
	if a.Extractor != nil {
		extraction = "pending"
	}
	if mail == nil {
		mail = &MailMeta{}
	}
	name := sanitizeName(filename, ext)
	var id int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO receipts(employee_id, source, sha256, filename, mime, size, pages, storage_key, preview_key, note, extraction, mail_message_id, mail_from, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.EmployeeID, source, sum, name, mime, size, pages, key, previewKey, strings.TrimSpace(note), extraction, mail.MessageID, mail.From, a.stamp())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		return a.audit(ctx, tx, u.ID, "receipt", id, "import", "", map[string]any{"source": source, "sha256": sum, "size": size, "filename": name})
	})
	if err != nil {
		a.Files.Remove(key)
		a.Files.Remove(previewKey)
		return nil, err
	}
	a.Log.Info("receipt imported", "receipt", id, "source", source, "file", name, "mime", mime, "bytes", size, "pages", pages)
	if a.Extractor != nil {
		a.enqueueExtraction(id)
	}
	return a.GetReceipt(ctx, u, id)
}

func sanitizeName(name, ext string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." {
		name = "beleg" + ext
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

// ReceiptInput is what the employee confirms or corrects.
type ReceiptInput struct {
	Date     string      `json:"date" doc:"YYYY-MM-DD or empty" maxLength:"10"`
	Total    money.Cents `json:"total" minimum:"0" maximum:"100000" doc:"cents; 0 when unknown"`
	Merchant string      `json:"merchant" maxLength:"200"`
	Note     string      `json:"note" maxLength:"500"`
}

func validDate(s string) error {
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return Invalid("receipt.date_invalid", "the date must be YYYY-MM-DD")
	}
	return nil
}

// monthOpen fails when month (YYYY-MM) is closed or paid.
func monthOpen(ctx context.Context, tx *sql.Tx, employeeID int64, month string) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM months WHERE employee_id = ? AND month = ?`, employeeID, month).Scan(&status)
	if err == sql.ErrNoRows || status == "open" {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrMonthClosed
}

// editable loads a receipt for change; a checked receipt of a closed month
// is frozen, and so is a target date in a closed month once checked.
func editable(ctx context.Context, tx *sql.Tx, u *User, id int64) (status, date string, err error) {
	var d sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT status, date FROM receipts WHERE id = ? AND employee_id = ?`, id, u.EmployeeID).Scan(&status, &d); err != nil {
		return "", "", notFound(err)
	}
	if status == "checked" && d.Valid {
		if err := monthOpen(ctx, tx, u.EmployeeID, d.String[:7]); err != nil {
			return "", "", err
		}
	}
	return status, d.String, nil
}

func nullable[T comparable](v T) any {
	var zero T
	if v == zero {
		return nil
	}
	return v
}

// UpdateReceipt saves the employee's corrections without changing the status.
func (a *App) UpdateReceipt(ctx context.Context, u *User, id int64, in ReceiptInput) (*Receipt, error) {
	if in.Date != "" {
		if err := validDate(in.Date); err != nil {
			return nil, err
		}
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		status, _, err := editable(ctx, tx, u, id)
		if err != nil {
			return err
		}
		if status == "checked" {
			if in.Date == "" || in.Total <= 0 {
				return Invalid("receipt.incomplete", "a checked receipt needs date and total")
			}
			if err := monthOpen(ctx, tx, u.EmployeeID, in.Date[:7]); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE receipts SET date = ?, total = ?, merchant = ?, note = ? WHERE id = ?`,
			nullable(in.Date), nullable(in.Total), strings.TrimSpace(in.Merchant), strings.TrimSpace(in.Note), id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "receipt", id, "update", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

// CheckReceipt saves the confirmed values and marks the receipt checked.
// The first check assigns a permanent number B-<year>-<seq>.
func (a *App) CheckReceipt(ctx context.Context, u *User, id int64, in ReceiptInput) (*Receipt, error) {
	if in.Date == "" || in.Total <= 0 {
		return nil, Invalid("receipt.incomplete", "date and total are required")
	}
	if err := validDate(in.Date); err != nil {
		return nil, err
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if _, _, err := editable(ctx, tx, u, id); err != nil {
			return err
		}
		if err := monthOpen(ctx, tx, u.EmployeeID, in.Date[:7]); err != nil {
			return err
		}
		var number sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT number FROM receipts WHERE id = ?`, id).Scan(&number); err != nil {
			return err
		}
		if !number.Valid {
			year := a.now().Year()
			var seq int
			if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(number_seq), 0) + 1 FROM receipts WHERE employee_id = ? AND number_year = ?`, u.EmployeeID, year).Scan(&seq); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE receipts SET number = ?, number_year = ?, number_seq = ? WHERE id = ?`, fmt.Sprintf("B-%d-%04d", year, seq), year, seq, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE receipts SET status = 'checked', date = ?, total = ?, merchant = ?, note = ?, checked_at = ? WHERE id = ?`,
			in.Date, in.Total, strings.TrimSpace(in.Merchant), strings.TrimSpace(in.Note), a.stamp(), id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "receipt", id, "check", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

// UncheckReceipt returns a checked receipt of an open month to pending.
func (a *App) UncheckReceipt(ctx context.Context, u *User, id int64) (*Receipt, error) {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if _, _, err := editable(ctx, tx, u, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE receipts SET status = 'pending', checked_at = NULL WHERE id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "receipt", id, "uncheck", "", nil)
	})
	if err != nil {
		return nil, err
	}
	return a.GetReceipt(ctx, u, id)
}

// DeleteReceipt removes a receipt that was never checked (wrong upload,
// duplicate). Checked receipts carry a number and stay as evidence.
func (a *App) DeleteReceipt(ctx context.Context, u *User, id int64) error {
	var key, preview string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var number sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT number, storage_key, preview_key FROM receipts WHERE id = ? AND employee_id = ?`, id, u.EmployeeID).Scan(&number, &key, &preview); err != nil {
			return notFound(err)
		}
		if number.Valid {
			return Invalid("receipt.numbered", "a receipt that was checked once stays as evidence")
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM receipts WHERE id = ?`, id); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "receipt", id, "delete", "", nil)
	})
	if err == nil {
		a.Files.Remove(key)
		a.Files.Remove(preview)
	}
	return err
}

// GetReceipt loads one owned receipt.
func (a *App) GetReceipt(ctx context.Context, u *User, id int64) (*Receipt, error) {
	list, err := a.loadReceipts(ctx, u.EmployeeID, "r.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

// ListReceipts lists receipts dated in month (YYYY-MM, "" for all) plus
// undated ones, optionally by status.
func (a *App) ListReceipts(ctx context.Context, u *User, month, status string) ([]*Receipt, error) {
	where, args := "1 = 1", []any{}
	if month != "" {
		where, args = "(r.date LIKE ? OR r.date IS NULL)", append(args, month+"-%")
	}
	if status != "" {
		where, args = where+" AND r.status = ?", append(args, status)
	}
	return a.loadReceipts(ctx, u.EmployeeID, where, args...)
}

func (a *App) loadReceipts(ctx context.Context, employeeID int64, where string, args ...any) ([]*Receipt, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT r.id, r.status, r.source, coalesce(r.number, ''), r.filename, r.mime, r.pages, coalesce(r.date, ''), coalesce(r.total, 0), r.merchant, r.note, r.mail_from, r.extraction, r.extraction_json, r.created_at, coalesce(r.checked_at, ''),
		EXISTS (SELECT 1 FROM receipts d WHERE d.employee_id = r.employee_id AND d.sha256 = r.sha256 AND d.id <> r.id),
		coalesce((SELECT m.status <> 'open' FROM months m WHERE m.employee_id = r.employee_id AND m.month = substr(r.date, 1, 7)), 0)
		FROM receipts r WHERE r.employee_id = ? AND `+where+` ORDER BY r.date IS NOT NULL, r.date DESC, r.id DESC`, append([]any{employeeID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := a.now().Format(time.DateOnly)
	out := []*Receipt{}
	for rows.Next() {
		r := &Receipt{}
		var extracted string
		var dup, closed bool
		if err := rows.Scan(&r.ID, &r.Status, &r.Source, &r.Number, &r.Filename, &r.Mime, &r.Pages, &r.Date, &r.Total, &r.Merchant, &r.Note, &r.MailFrom, &r.Extraction, &extracted, &r.CreatedAt, &r.CheckedAt, &dup, &closed); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(extracted), &r.Extracted)
		r.Warnings = receiptWarnings(r, dup, closed, today)
		out = append(out, r)
	}
	return out, rows.Err()
}

func receiptWarnings(r *Receipt, dup, closed bool, today string) []string {
	w := []string{}
	if dup {
		w = append(w, "duplicate")
	}
	if r.Date == "" {
		w = append(w, "date_missing")
	} else if r.Date > today {
		w = append(w, "future_date")
	}
	if r.Total == 0 {
		w = append(w, "total_missing")
	}
	if closed && r.Status == "pending" {
		w = append(w, "month_closed")
	}
	return w
}

// ReceiptFile returns an owned receipt's original or preview bytes.
func (a *App) ReceiptFile(ctx context.Context, u *User, id int64, preview bool) ([]byte, string, string, error) {
	var key, previewKey, mime, name string
	err := a.DB.QueryRowContext(ctx, `SELECT storage_key, preview_key, mime, filename FROM receipts WHERE id = ? AND employee_id = ?`, id, u.EmployeeID).Scan(&key, &previewKey, &mime, &name)
	if err != nil {
		return nil, "", "", notFound(err)
	}
	if preview {
		key, mime = previewKey, "image/png"
	}
	b, err := a.Files.Read(key)
	return b, mime, name, err
}

// MailImported reports whether a mail was imported before.
func (a *App) MailImported(ctx context.Context, u *User, messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}
	var n int
	err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM receipts WHERE employee_id = ? AND mail_message_id = ?`, u.EmployeeID, messageID).Scan(&n)
	return n > 0, err
}
