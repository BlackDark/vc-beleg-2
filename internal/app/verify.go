package app

import (
	"context"
	"fmt"
)

// VerifyReport summarizes an integrity check of stored receipts and reports.
type VerifyReport struct {
	Receipts int      `json:"receipts"`
	Reports  int      `json:"reports"`
	Problems []string `json:"problems"`
}

// Verify recomputes the SHA-256 of every stored receipt original and every
// report PDF and compares it with the database (backup/restore check).
func (a *App) Verify(ctx context.Context) (*VerifyReport, error) {
	r := &VerifyReport{Problems: []string{}}
	type item struct{ label, key, sum string }
	var items []item
	rows, err := a.DB.QueryContext(ctx, `SELECT coalesce(number, 'receipt #' || id), storage_key, sha256 FROM receipts`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.label, &it.key, &it.sum); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, it)
		r.Receipts++
	}
	rows.Close()
	rows, err = a.DB.QueryContext(ctx, `SELECT m.month, r.version, r.pdf_key, r.pdf_sha256 FROM reports r JOIN months m ON m.id = r.month_id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var month string
		var version int
		var it item
		if err := rows.Scan(&month, &version, &it.key, &it.sum); err != nil {
			rows.Close()
			return nil, err
		}
		it.label = fmt.Sprintf("report %s v%d", month, version)
		items = append(items, it)
		r.Reports++
	}
	rows.Close()
	for _, it := range items {
		sum, err := a.Files.SHA256(it.key)
		switch {
		case err != nil:
			r.Problems = append(r.Problems, fmt.Sprintf("%s: %v", it.label, err))
		case sum != it.sum:
			r.Problems = append(r.Problems, fmt.Sprintf("%s: checksum mismatch", it.label))
		}
	}
	return r, nil
}
