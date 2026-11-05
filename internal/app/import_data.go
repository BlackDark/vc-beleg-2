package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/blackdark/vc-beleg-2/internal/export"
)

// ImportData creates one pending receipt per entry of a JSON data file
// (source "data", reading off, no extraction job). The canonical JSON record
// is the stored original and its SHA-256 drives duplicate detection.
//
// A batch is all-or-nothing: when any entry is invalid nothing is created and
// the returned error lists every invalid entry (Error.Entries).
func (a *App) ImportData(ctx context.Context, u *User, filename string, data []byte) ([]*Receipt, error) {
	list, err := a.importData(ctx, u, filename, data)
	if err != nil {
		a.recordImport(ctx, u, "upload", filename, nil, err)
		return nil, err
	}
	for _, r := range list {
		a.recordImport(ctx, u, "upload", r.Filename, r, nil)
	}
	return list, nil
}

type storedData struct {
	entry           DataEntry
	key, previewKey string
	sum, name       string
	size            int64
}

func (a *App) importData(ctx context.Context, u *User, filename string, data []byte) ([]*Receipt, error) {
	if int64(len(data)) > a.Cfg.MaxUploadBytes {
		return nil, Invalid("upload.too_large", "file exceeds the size limit", "limitMb", fmt.Sprint(a.Cfg.MaxUploadBytes>>20))
	}
	entries, entryErrs, err := ParseDataFile(data)
	if err != nil {
		return nil, err
	}
	if len(entryErrs) > 0 {
		e := Invalid("data.invalid", "the file has invalid entries; nothing was imported")
		e.Entries = entryErrs
		return nil, e
	}
	stored := make([]storedData, 0, len(entries))
	cleanup := func() {
		for _, s := range stored {
			a.Files.Remove(s.key)
			a.Files.Remove(s.previewKey)
		}
	}
	for i, e := range entries {
		png, err := a.Typst.DataReceiptPreview(ctx, e.Canonical, 600)
		if err != nil {
			cleanup()
			return nil, Invalid("data.unreadable", "the data receipt could not be rendered", "entry", fmt.Sprint(i+1))
		}
		s := storedData{entry: e, name: dataFilename(filename, i, len(entries))}
		if s.key, s.sum, s.size, err = a.Files.Put("files", ".json", bytes.NewReader(e.Canonical), a.Cfg.MaxUploadBytes); err != nil {
			cleanup()
			return nil, err
		}
		// Registered before the preview is stored so a failure removes the original too.
		stored = append(stored, s)
		if stored[i].previewKey, _, _, err = a.Files.Put("files", ".png", bytes.NewReader(png), 1<<26); err != nil {
			cleanup()
			return nil, err
		}
	}
	ids := make([]int64, 0, len(stored))
	err = a.tx(ctx, func(tx *sql.Tx) error {
		ids = ids[:0]
		for _, s := range stored {
			res, err := tx.ExecContext(ctx, `INSERT INTO receipts(employee_id, source, sha256, filename, mime, size, pages, storage_key, preview_key, date, total, merchant, extraction, created_at) VALUES (?, 'data', ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, 'off', ?)`,
				u.EmployeeID, s.sum, s.name, export.DataMime, s.size, s.key, s.previewKey, s.entry.Date, s.entry.Total, s.entry.Merchant, a.stamp())
			if err != nil {
				return err
			}
			id, _ := res.LastInsertId()
			ids = append(ids, id)
			if err := a.audit(ctx, tx, u.ID, "receipt", id, "import", "", map[string]any{"source": "data", "sha256": s.sum, "size": s.size, "filename": s.name}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		cleanup()
		return nil, err
	}
	a.Log.Info("data receipts imported", "file", filename, "receipts", len(ids))
	out := make([]*Receipt, 0, len(ids))
	for _, id := range ids {
		r, err := a.GetReceipt(ctx, u, id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// dataFilename names the receipt of entry i; entries of an array get a
// position so they can be told apart.
func dataFilename(filename string, i, total int) string {
	name := sanitizeName(filename, ".json")
	if total == 1 {
		return name
	}
	return fmt.Sprintf("%s (%d)", strings.TrimSuffix(name, ".json"), i+1) + ".json"
}
