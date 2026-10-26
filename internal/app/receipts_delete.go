package app

import (
	"context"
	"database/sql"
)

// archivedReceiptSQL checks every report version, independently of the receipt's
// current date or status. Snapshots also list receipts not appended to the PDF.
const archivedReceiptSQL = `EXISTS (
 SELECT 1 FROM reports p JOIN months m ON m.id = p.month_id,
 json_each(p.snapshot_json, '$.receipts') sr
 WHERE m.employee_id = r.employee_id AND json_extract(sr.value, '$.id') = r.id
)`

// DeleteReceipt removes receipts that have never appeared in an archived report.
func (a *App) DeleteReceipt(ctx context.Context, u *User, id int64) error {
	var key, preview string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var archived bool
		if err := tx.QueryRowContext(ctx, `SELECT storage_key, preview_key, `+archivedReceiptSQL+` FROM receipts r WHERE r.id = ? AND r.employee_id = ?`, id, u.EmployeeID).Scan(&key, &preview, &archived); err != nil {
			return notFound(err)
		}
		if archived {
			return Invalid("receipt.archived", "a receipt included in an archived report stays as evidence")
		}
		if _, _, err := editable(ctx, tx, u, id); err != nil {
			return err
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
