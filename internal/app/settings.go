package app

import (
	"context"
	"database/sql"
	"strings"

	"github.com/blackdark/vc-beleg-2/internal/calc"
	"github.com/blackdark/vc-beleg-2/internal/holidays"
)

// Settings are the company and employee data of the account.
type Settings struct {
	CompanyName    string `json:"companyName" maxLength:"200"`
	CompanyAddress string `json:"companyAddress" maxLength:"500"`
	State          string `json:"state" enum:"BW,BY,BE,BB,HB,HH,HE,MV,NI,NW,RP,SL,SN,ST,SH,TH" doc:"Bundesland for public holidays"`
	ChurchTaxBP    int64  `json:"churchTaxBp" minimum:"0" maximum:"1000" doc:"church tax on the flat tax in basis points; 0 with proof of non-membership"`
	LegalBasis     string `json:"legalBasis" maxLength:"500" doc:"printed on every report, e.g. a shareholder resolution"`
	EmployeeName   string `json:"employeeName" minLength:"1" maxLength:"200"`
	TaxModel       string `json:"taxModel" enum:"A,B,C"`
}

// employeeSettings is what a month's calculation needs.
type employeeSettings struct {
	Settings
	EmployeeID int64
}

func (a *App) loadSettings(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, employeeID int64) (*employeeSettings, error) {
	s := &employeeSettings{EmployeeID: employeeID}
	err := q.QueryRowContext(ctx, `SELECT c.name, c.address, c.state, c.church_tax_bp, c.legal_basis, e.name, e.tax_model FROM employees e JOIN companies c ON c.id = e.company_id WHERE e.id = ?`, employeeID).
		Scan(&s.CompanyName, &s.CompanyAddress, &s.State, &s.ChurchTaxBP, &s.LegalBasis, &s.EmployeeName, &s.TaxModel)
	return s, notFound(err)
}

// GetSettings returns the account's settings.
func (a *App) GetSettings(ctx context.Context, u *User) (*Settings, error) {
	s, err := a.loadSettings(ctx, a.DB, u.EmployeeID)
	if err != nil {
		return nil, err
	}
	return &s.Settings, nil
}

// SaveSettings stores company and employee data. Closed months keep the
// values frozen in their report.
func (a *App) SaveSettings(ctx context.Context, u *User, in Settings) (*Settings, error) {
	if !holidays.ValidState(in.State) {
		return nil, Invalid("settings.state", "unknown Bundesland")
	}
	if !calc.TaxModel(in.TaxModel).Valid() {
		return nil, Invalid("settings.tax_model", "unknown tax model")
	}
	if strings.TrimSpace(in.EmployeeName) == "" {
		return nil, Invalid("settings.employee_name", "the employee name is required")
	}
	err := a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE companies SET name = ?, address = ?, state = ?, church_tax_bp = ?, legal_basis = ? WHERE id = (SELECT company_id FROM employees WHERE id = ?)`,
			strings.TrimSpace(in.CompanyName), strings.TrimSpace(in.CompanyAddress), in.State, in.ChurchTaxBP, strings.TrimSpace(in.LegalBasis), u.EmployeeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE employees SET name = ?, tax_model = ? WHERE id = ?`, strings.TrimSpace(in.EmployeeName), in.TaxModel, u.EmployeeID); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, "settings", u.EmployeeID, "save", "", in)
	})
	if err != nil {
		return nil, err
	}
	return a.GetSettings(ctx, u)
}
