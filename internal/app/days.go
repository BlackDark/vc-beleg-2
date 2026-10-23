package app

import (
	"context"
	"database/sql"

	"github.com/blackdark/vc-beleg-2/internal/calc"
)

// Day-level choices of an open month: combined receipts, meal, working-day
// calendar. Each returns the recalculated month.

func loadMarks(ctx context.Context, q querier, employeeID int64, month string) (map[string]calc.Mark, error) {
	rows, err := q.QueryContext(ctx, `SELECT date, mark FROM day_marks WHERE employee_id = ? AND date LIKE ?`, employeeID, month+"-%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]calc.Mark{}
	for rows.Next() {
		var d, mark string
		if err := rows.Scan(&d, &mark); err != nil {
			return nil, err
		}
		out[d] = calc.Mark(mark)
	}
	return out, rows.Err()
}

func loadMeals(ctx context.Context, q querier, employeeID int64, month string, into map[string]calc.Meal) error {
	rows, err := q.QueryContext(ctx, `SELECT date, meal FROM day_meals WHERE employee_id = ? AND date LIKE ?`, employeeID, month+"-%")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d, meal string
		if err := rows.Scan(&d, &meal); err != nil {
			return err
		}
		into[d] = calc.Meal(meal)
	}
	return rows.Err()
}

// SetCombined combines the receipts of a date into one meal, or undoes it.
func (a *App) SetCombined(ctx context.Context, u *User, date string, combined bool) (*Month, error) {
	if err := validDate(date); err != nil {
		return nil, err
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if err := monthOpen(ctx, tx, u.EmployeeID, date[:7]); err != nil {
			return err
		}
		q := `DELETE FROM combined_days WHERE employee_id = ? AND date = ?`
		if combined {
			q = `INSERT OR IGNORE INTO combined_days(employee_id, date) VALUES (?, ?)`
		}
		if _, err := tx.ExecContext(ctx, q, u.EmployeeID, date); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "day", u.EmployeeID, "combine", "", map[string]any{"date": date, "combined": combined})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, date[:7])
}

// SetMeal sets the meal a date's allowance is for; lunch is the default.
func (a *App) SetMeal(ctx context.Context, u *User, date string, meal calc.Meal) (*Month, error) {
	if err := validDate(date); err != nil {
		return nil, err
	}
	if !meal.Valid() {
		return nil, Invalid("day.meal", "the meal must be breakfast, lunch or dinner")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if err := monthOpen(ctx, tx, u.EmployeeID, date[:7]); err != nil {
			return err
		}
		var err error
		if meal == calc.Lunch {
			_, err = tx.ExecContext(ctx, `DELETE FROM day_meals WHERE employee_id = ? AND date = ?`, u.EmployeeID, date)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO day_meals(employee_id, date, meal) VALUES (?, ?, ?) ON CONFLICT(employee_id, date) DO UPDATE SET meal = excluded.meal`, u.EmployeeID, date, string(meal))
		}
		if err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "day", u.EmployeeID, "meal", "", map[string]any{"date": date, "meal": meal})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, date[:7])
}

// SetProveDays turns the working-day proof of an open month on or off.
func (a *App) SetProveDays(ctx context.Context, u *User, month string, prove bool) (*Month, error) {
	if err := validMonth(month); err != nil {
		return nil, err
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if err := monthOpen(ctx, tx, u.EmployeeID, month); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO months(employee_id, month, status, prove_working_days) VALUES (?, ?, 'open', ?)
			ON CONFLICT(employee_id, month) DO UPDATE SET prove_working_days = excluded.prove_working_days`, u.EmployeeID, month, prove); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "month", u.EmployeeID, "prove_days", "", map[string]any{"month": month, "prove": prove})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, month)
}

// SetMark sets a date's working-day calendar entry, or clears it ("").
func (a *App) SetMark(ctx context.Context, u *User, date string, mark calc.Mark) (*Month, error) {
	if err := validDate(date); err != nil {
		return nil, err
	}
	if mark != "" && !mark.Valid() {
		return nil, Invalid("day.mark", "the mark must be vacation, sick, off, worked or empty")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if err := monthOpen(ctx, tx, u.EmployeeID, date[:7]); err != nil {
			return err
		}
		var err error
		if mark == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM day_marks WHERE employee_id = ? AND date = ?`, u.EmployeeID, date)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO day_marks(employee_id, date, mark) VALUES (?, ?, ?) ON CONFLICT(employee_id, date) DO UPDATE SET mark = excluded.mark`, u.EmployeeID, date, string(mark))
		}
		if err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "day", u.EmployeeID, "mark", "", map[string]any{"date": date, "mark": mark})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, date[:7])
}
