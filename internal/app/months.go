package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/calc"
	"github.com/blackdark/vc-beleg-2/internal/export"
	"github.com/blackdark/vc-beleg-2/internal/holidays"
	"github.com/blackdark/vc-beleg-2/internal/money"
)

// ReportRef is one generated report version.
type ReportRef struct {
	ID           int64       `json:"id"`
	Version      int         `json:"version"`
	Payout       money.Cents `json:"payout"`
	ReopenReason string      `json:"reopenReason" doc:"why the previous version was reopened"`
	CreatedAt    string      `json:"createdAt"`
}

// Month is one calendar month of the employee with its calculation.
type Month struct {
	Month        string      `json:"month"`
	Status       string      `json:"status" enum:"open,closed,paid"`
	PaidOn       string      `json:"paidOn"`
	ReopenReason string      `json:"reopenReason" doc:"set while a reopened month is open again"`
	Result       calc.Result `json:"result" doc:"live for open months, frozen from the latest report otherwise"`
	Pending      int         `json:"pending" doc:"receipts dated in the month that are still to check"`
	Undated      int         `json:"undated" doc:"receipts to check without a date"`
	Blockers     []string    `json:"blockers" doc:"values_missing, pending_receipts, future_month"`
	Reports      []ReportRef `json:"reports"`
}

// MonthSummary is a row of the month overview.
type MonthSummary struct {
	Month   string      `json:"month"`
	Status  string      `json:"status" enum:"open,closed,paid"`
	PaidOn  string      `json:"paidOn"`
	Payout  money.Cents `json:"payout"`
	Reports []ReportRef `json:"reports"`
}

type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validMonth(m string) error {
	if _, err := time.Parse("2006-01", m); err != nil || len(m) != 7 {
		return Invalid("month.invalid", "the month must be YYYY-MM")
	}
	return nil
}

// compute runs the calculation for an open month from the stored data.
func (a *App) compute(ctx context.Context, q querier, s *employeeSettings, month string) (calc.Result, error) {
	in := calc.Input{Month: month, Model: calc.TaxModel(s.TaxModel), DayLimit: calc.DefaultDayLimit, Combined: map[string]bool{}, ChurchTaxBP: s.ChurchTaxBP}
	year, _ := strconv.Atoi(month[:4])
	if v, ok := a.Rates[year]; ok {
		in.Values = &v
	}
	in.Holidays = holidays.Year(s.State, year)
	rows, err := q.QueryContext(ctx, `SELECT id, date, total FROM receipts WHERE employee_id = ? AND status = 'checked' AND date LIKE ?`, s.EmployeeID, month+"-%")
	if err != nil {
		return calc.Result{}, err
	}
	for rows.Next() {
		var r calc.Receipt
		if err := rows.Scan(&r.ID, &r.Date, &r.Total); err != nil {
			rows.Close()
			return calc.Result{}, err
		}
		in.Receipts = append(in.Receipts, r)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT date FROM combined_days WHERE employee_id = ? AND date LIKE ?`, s.EmployeeID, month+"-%")
	if err != nil {
		return calc.Result{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return calc.Result{}, err
		}
		in.Combined[d] = true
	}
	return calc.Compute(in), rows.Err()
}

func (a *App) reportRefs(ctx context.Context, q querier, monthID int64) ([]ReportRef, error) {
	out := []ReportRef{}
	rows, err := q.QueryContext(ctx, `SELECT id, version, payout, reopen_reason, created_at FROM reports WHERE month_id = ? ORDER BY version DESC`, monthID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ReportRef
		if err := rows.Scan(&r.ID, &r.Version, &r.Payout, &r.ReopenReason, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetMonth returns a month with its calculation and what blocks closing it.
func (a *App) GetMonth(ctx context.Context, u *User, month string) (*Month, error) {
	if err := validMonth(month); err != nil {
		return nil, err
	}
	return a.loadMonth(ctx, a.DB, u, month)
}

func (a *App) loadMonth(ctx context.Context, q querier, u *User, month string) (*Month, error) {
	m := &Month{Month: month, Status: "open", Blockers: []string{}, Reports: []ReportRef{}}
	var id int64
	var paid sql.NullString
	err := q.QueryRowContext(ctx, `SELECT id, status, paid_on, reopen_reason FROM months WHERE employee_id = ? AND month = ?`, u.EmployeeID, month).Scan(&id, &m.Status, &paid, &m.ReopenReason)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	m.PaidOn = paid.String
	if id != 0 {
		if m.Reports, err = a.reportRefs(ctx, q, id); err != nil {
			return nil, err
		}
	}
	if err := q.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE date LIKE ?), count(*) FILTER (WHERE date IS NULL) FROM receipts WHERE employee_id = ? AND status = 'pending'`, month+"-%", u.EmployeeID).Scan(&m.Pending, &m.Undated); err != nil {
		return nil, err
	}
	if m.Status != "open" {
		var snap string
		if err := q.QueryRowContext(ctx, `SELECT snapshot_json FROM reports WHERE month_id = ? ORDER BY version DESC LIMIT 1`, id).Scan(&snap); err != nil {
			return nil, err
		}
		var s export.Snapshot
		if err := json.Unmarshal([]byte(snap), &s); err != nil {
			return nil, err
		}
		m.Result = s.Result
		return m, nil
	}
	s, err := a.loadSettings(ctx, q, u.EmployeeID)
	if err != nil {
		return nil, err
	}
	if m.Result, err = a.compute(ctx, q, s, month); err != nil {
		return nil, err
	}
	m.Blockers = append(m.Blockers, m.Result.Blockers...)
	if m.Pending > 0 {
		m.Blockers = append(m.Blockers, "pending_receipts")
	}
	if month > a.now().Format("2006-01") {
		m.Blockers = append(m.Blockers, "future_month")
	}
	return m, nil
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

// ListMonths returns every month that has receipts or a report, newest first.
func (a *App) ListMonths(ctx context.Context, u *User) ([]MonthSummary, error) {
	rows, err := a.DB.QueryContext(ctx, `SELECT month FROM (
		SELECT DISTINCT substr(date, 1, 7) AS month FROM receipts WHERE employee_id = ? AND date IS NOT NULL
		UNION SELECT month FROM months WHERE employee_id = ?) ORDER BY month DESC`, u.EmployeeID, u.EmployeeID)
	if err != nil {
		return nil, err
	}
	var months []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			rows.Close()
			return nil, err
		}
		months = append(months, m)
	}
	rows.Close()
	out := []MonthSummary{}
	for _, month := range months {
		m, err := a.loadMonth(ctx, a.DB, u, month)
		if err != nil {
			return nil, err
		}
		out = append(out, MonthSummary{Month: m.Month, Status: m.Status, PaidOn: m.PaidOn, Payout: m.Result.Totals.Allowance, Reports: m.Reports})
	}
	return out, nil
}

// CloseMonth freezes the calculation and generates the next report version.
func (a *App) CloseMonth(ctx context.Context, u *User, month string) (*Month, error) {
	if err := validMonth(month); err != nil {
		return nil, err
	}
	var pdfKey string
	err := a.tx(ctx, func(tx *sql.Tx) error {
		m, err := a.loadMonth(ctx, tx, u, month)
		if err != nil {
			return err
		}
		if m.Status != "open" {
			return Invalid("month.not_open", "the month is already closed")
		}
		if len(m.Blockers) > 0 {
			return Invalid("month.blocked", "the month cannot be closed yet", "blockers", strings.Join(m.Blockers, ","))
		}
		s, err := a.loadSettings(ctx, tx, u.EmployeeID)
		if err != nil {
			return err
		}
		snap, files, err := a.snapshot(ctx, tx, s, m, len(m.Reports)+1)
		if err != nil {
			return err
		}
		pdf, err := export.RenderMonth(ctx, a.Typst, snap, files)
		if err != nil {
			return err
		}
		var sum string
		pdfKey, sum, _, err = a.Files.Put("exports", ".pdf", strings.NewReader(string(pdf)), 1<<30)
		if err != nil {
			return err
		}
		b, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO months(employee_id, month, status) VALUES (?, ?, 'closed') ON CONFLICT(employee_id, month) DO UPDATE SET status = 'closed', reopen_reason = ''`, u.EmployeeID, month); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO reports(month_id, version, snapshot_json, payout, pdf_key, pdf_sha256, reopen_reason, created_at)
			SELECT id, ?, ?, ?, ?, ?, ?, ? FROM months WHERE employee_id = ? AND month = ?`,
			snap.Version, string(b), m.Result.Totals.Allowance, pdfKey, sum, m.ReopenReason, a.stamp(), u.EmployeeID, month)
		if err != nil {
			return err
		}
		reportID, _ := res.LastInsertId()
		return a.audit(ctx, tx, u.ID, "month", reportID, "close", "", map[string]any{"month": month, "version": snap.Version, "pdfSha256": sum})
	})
	if err != nil {
		if pdfKey != "" {
			a.Files.Remove(pdfKey)
		}
		return nil, err
	}
	return a.GetMonth(ctx, u, month)
}

// ReopenMonth makes a closed month editable again. Its reports stay
// unchanged; the next close creates a new version naming the reason.
func (a *App) ReopenMonth(ctx context.Context, u *User, month, reason string) (*Month, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, Invalid("month.reason", "a reason is required")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE months SET status = 'open', reopen_reason = ? WHERE employee_id = ? AND month = ? AND status = 'closed'`, strings.TrimSpace(reason), u.EmployeeID, month)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Invalid("month.not_closed", "only a closed, unpaid month can be reopened")
		}
		return a.audit(ctx, tx, u.ID, "month", u.EmployeeID, "reopen", reason, map[string]any{"month": month})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, month)
}

// SetPaid records the payout date of a closed month, or clears it ("").
func (a *App) SetPaid(ctx context.Context, u *User, month, paidOn string) (*Month, error) {
	if paidOn != "" {
		if err := validDate(paidOn); err != nil {
			return nil, err
		}
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		from, to := "closed", "paid"
		if paidOn == "" {
			from, to = "paid", "closed"
		}
		res, err := tx.ExecContext(ctx, `UPDATE months SET status = ?, paid_on = ? WHERE employee_id = ? AND month = ? AND status IN (?, 'paid')`, to, nullable(paidOn), u.EmployeeID, month, from)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Invalid("month.not_closed", "close the month first")
		}
		return a.audit(ctx, tx, u.ID, "month", u.EmployeeID, "paid", "", map[string]any{"month": month, "paidOn": paidOn})
	})
	if err != nil {
		return nil, err
	}
	return a.GetMonth(ctx, u, month)
}

// ReportFile returns a report PDF and its file name.
func (a *App) ReportFile(ctx context.Context, u *User, id int64) ([]byte, string, error) {
	var key, month string
	var version int
	err := a.DB.QueryRowContext(ctx, `SELECT r.pdf_key, m.month, r.version FROM reports r JOIN months m ON m.id = r.month_id WHERE r.id = ? AND m.employee_id = ?`, id, u.EmployeeID).Scan(&key, &month, &version)
	if err != nil {
		return nil, "", notFound(err)
	}
	b, err := a.Files.Read(key)
	return b, "Essenszuschuss-" + month + "-v" + strconv.Itoa(version) + ".pdf", err
}
