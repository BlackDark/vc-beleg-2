package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/export"
)

// snapshot freezes the month for its report and loads the originals of the
// receipts that make up counted days.
func (a *App) snapshot(ctx context.Context, tx *sql.Tx, s *employeeSettings, m *Month, version int) (export.Snapshot, map[int64][]byte, error) {
	snap := export.Snapshot{
		Version: version, Month: m.Month, ReopenReason: m.ReopenReason,
		GeneratedAt:  a.now().In(berlin).Format("02.01.2006 15:04"),
		Company:      export.SnapshotCompany{Name: s.CompanyName, Address: s.CompanyAddress, State: s.State, ChurchTaxBP: s.ChurchTaxBP, LegalBasis: s.LegalBasis},
		EmployeeName: s.EmployeeName, Result: m.Result, Receipts: []export.SnapReceipt{},
	}
	appended := map[int64]bool{}
	for _, d := range m.Result.Days {
		for _, id := range d.ReceiptIDs {
			appended[id] = d.Counted
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, number, date, merchant, total, filename, mime, pages, sha256, storage_key FROM receipts
		WHERE employee_id = ? AND status = 'checked' AND date LIKE ? ORDER BY date, id`, s.EmployeeID, m.Month+"-%")
	if err != nil {
		return snap, nil, err
	}
	keys := map[int64]string{}
	for rows.Next() {
		var r export.SnapReceipt
		var key string
		if err := rows.Scan(&r.ID, &r.Number, &r.Date, &r.Merchant, &r.Total, &r.Filename, &r.Mime, &r.Pages, &r.SHA256, &key); err != nil {
			rows.Close()
			return snap, nil, err
		}
		r.Appended = appended[r.ID]
		if r.Appended {
			keys[r.ID] = key
		}
		snap.Receipts = append(snap.Receipts, r)
	}
	rows.Close()
	files := map[int64][]byte{}
	for id, key := range keys {
		b, err := a.Files.Read(key)
		if err != nil {
			return snap, nil, err
		}
		files[id] = b
	}
	return snap, files, rows.Err()
}

// berlin is the report's time zone; the image ships tzdata.
var berlin = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.UTC
	}
	return loc
}()
