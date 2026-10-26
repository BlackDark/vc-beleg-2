package app

import (
	"context"
	"database/sql"

	"github.com/blackdark/vc-beleg-2/internal/calc"
)

// SetSelectedReceipt chooses the checked receipt for a meal day; 0 restores automatic selection.
func (a *App) SetSelectedReceipt(ctx context.Context, u *User, date string, receiptID int64) (*Month, error) {
	if err := validDate(date); err != nil {
		return nil, err
	}
	if receiptID < 0 {
		return nil, Invalid("day.receipt_invalid", "choose a checked receipt from this day")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if err := monthOpen(ctx, tx, u.EmployeeID, date[:7]); err != nil {
			return err
		}
		if receiptID != 0 {
			var eligible bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM receipts WHERE id = ? AND employee_id = ? AND date = ? AND status = 'checked')`, receiptID, u.EmployeeID, date).Scan(&eligible); err != nil {
				return err
			}
			if !eligible {
				return Invalid("day.receipt_invalid", "choose a checked receipt from this day")
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM combined_days WHERE employee_id = ? AND date = ?`, u.EmployeeID, date); err != nil {
			return err
		}
		if receiptID == 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM selected_receipts WHERE employee_id = ? AND date = ?`, u.EmployeeID, date); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `INSERT INTO selected_receipts(employee_id, date, receipt_id) VALUES (?, ?, ?) ON CONFLICT(employee_id, date) DO UPDATE SET receipt_id = excluded.receipt_id`, u.EmployeeID, date, receiptID); err != nil {
				return err
			}
		}
		return a.audit(ctx, tx, u.ID, "day", u.EmployeeID, "select_receipt", "", map[string]any{"date": date, "receiptId": receiptID})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, date[:7])
}

func loadDayChoices(ctx context.Context, q querier, employeeID int64, in *calc.Input) error {
	in.Combined, in.Selected = map[string]bool{}, map[string]int64{}
	rows, err := q.QueryContext(ctx, `SELECT date FROM combined_days WHERE employee_id = ? AND date LIKE ?`, employeeID, in.Month+"-%")
	if err != nil {
		return err
	}
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			rows.Close()
			return err
		}
		in.Combined[date] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = q.QueryContext(ctx, `SELECT date, receipt_id FROM selected_receipts WHERE employee_id = ? AND date LIKE ?`, employeeID, in.Month+"-%")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var id int64
		if err := rows.Scan(&date, &id); err != nil {
			return err
		}
		in.Selected[date] = id
	}
	return rows.Err()
}
