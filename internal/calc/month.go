// Package calc computes a month's meal allowances. It is the only place for
// tax rules; functions are pure and every day line names the rules it used
// (IDs from docs/CALCULATION_RULES.md).
package calc

import (
	"cmp"
	"slices"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/money"
	"github.com/blackdark/vc-beleg-2/internal/rates"
)

// TaxModel says how the taxable part of an allowance is taxed.
type TaxModel string

// Tax models (Versteuerungsmodell).
const (
	ModelFlat       TaxModel = "A" // company pays 25 % flat tax
	ModelIndividual TaxModel = "B" // taxed through payroll
	ModelTaxFree    TaxModel = "C" // allowance up to the top-up only
)

// Valid reports whether m is a known model.
func (m TaxModel) Valid() bool {
	return m == ModelFlat || m == ModelIndividual || m == ModelTaxFree
}

// Rates in basis points.
const (
	flatTaxBP = 2500 // § 40 Abs. 2 Satz 1 Nr. 1 EStG
	soliBP    = 550  // § 4 SolZG
)

// Meal is the meal an allowance is granted for (R1).
type Meal string

// Meals; lunch is the default.
const (
	Breakfast Meal = "breakfast"
	Lunch     Meal = "lunch"
	Dinner    Meal = "dinner"
)

// Valid reports whether m is a known meal.
func (m Meal) Valid() bool { return m == Breakfast || m == Lunch || m == Dinner }

// benefitValue is the meal's official value for the year.
func benefitValue(m Meal, v rates.Values) money.Cents {
	switch m {
	case Breakfast:
		return v.Breakfast
	case Dinner:
		return v.Dinner
	default:
		return v.Lunch
	}
}

// DefaultDayLimit is the 15-day simplification (R4).
const DefaultDayLimit = 15

// Receipt is a checked receipt dated in the month.
type Receipt struct {
	ID    int64
	Date  string // YYYY-MM-DD
	Total money.Cents
}

// Input is everything one month's calculation depends on.
type Input struct {
	Month       string // YYYY-MM
	Model       TaxModel
	DayLimit    int
	Values      *rates.Values // nil when the year has no values
	Receipts    []Receipt
	Combined    map[string]bool   // dates whose receipts are combined into one meal
	Meals       map[string]Meal   // dates with a meal other than lunch
	Holidays    map[string]string // date → name, for the company's state
	ChurchTaxBP int64             // church tax on the flat tax; 0 with proof of non-membership
}

// Day is one meal day (Essenstag).
type Day struct {
	Date       string      `json:"date"`
	ReceiptIDs []int64     `json:"receiptIds" doc:"receipts making up the day's total"`
	Ignored    []int64     `json:"ignored" doc:"further receipts of the day that do not count"`
	Combined   bool        `json:"combined"`
	Meal       Meal        `json:"meal" enum:"breakfast,lunch,dinner"`
	Total      money.Cents `json:"total"`
	Counted    bool        `json:"counted"`
	Allowance  money.Cents `json:"allowance"`
	TaxFree    money.Cents `json:"taxFree"`
	Taxable    money.Cents `json:"taxable"`
	Rules      []string    `json:"rules"`
	Warnings   []string    `json:"warnings" doc:"weekend, holiday, several_receipts, over_limit"`
	Holiday    string      `json:"holiday,omitempty"`
}

// Totals sum the counted days.
type Totals struct {
	Days       int         `json:"days"`
	Receipts   money.Cents `json:"receipts" doc:"sum of the counted days' receipt totals"`
	Allowance  money.Cents `json:"allowance" doc:"payout to the employee"`
	TaxFree    money.Cents `json:"taxFree"`
	Taxable    money.Cents `json:"taxable"`
	FlatTax    money.Cents `json:"flatTax" doc:"model A: 25 % wage tax paid by the company"`
	Soli       money.Cents `json:"soli"`
	ChurchTax  money.Cents `json:"churchTax"`
	CompanyTax money.Cents `json:"companyTax" doc:"flat tax + Soli + church tax"`
}

// Result is a computed month.
type Result struct {
	Month    string        `json:"month"`
	Model    TaxModel      `json:"model"`
	DayLimit int           `json:"dayLimit"`
	Values   *rates.Values `json:"values,omitempty"`
	Days     []Day         `json:"days"`
	Totals   Totals        `json:"totals"`
	Blockers []string      `json:"blockers" doc:"values_missing"`
}

// Compute calculates a month. Receipts outside the month are ignored.
func Compute(in Input) Result {
	res := Result{Month: in.Month, Model: in.Model, DayLimit: in.DayLimit, Values: in.Values, Days: []Day{}, Blockers: []string{}}
	if res.DayLimit <= 0 {
		res.DayLimit = DefaultDayLimit
	}
	if in.Values == nil {
		res.Blockers = append(res.Blockers, "values_missing")
	}
	res.Days = mealDays(in)
	countBest(res.Days, res.DayLimit)
	if in.Values != nil {
		for i := range res.Days {
			if res.Days[i].Counted {
				split(&res.Days[i], in.Model, *in.Values)
			}
		}
	}
	res.Totals = sum(res.Days, in.Model, in.ChurchTaxBP)
	return res
}

// mealDays groups receipts by date (R1). Per day the highest receipt counts,
// or all of them when the employee combined the day.
func mealDays(in Input) []Day {
	byDate := map[string][]Receipt{}
	for _, r := range in.Receipts {
		if len(r.Date) == 10 && r.Date[:7] == in.Month {
			byDate[r.Date] = append(byDate[r.Date], r)
		}
	}
	days := make([]Day, 0, len(byDate))
	for date, rs := range byDate {
		slices.SortFunc(rs, func(a, b Receipt) int {
			return cmp.Or(cmp.Compare(b.Total, a.Total), cmp.Compare(a.ID, b.ID))
		})
		d := Day{Date: date, ReceiptIDs: []int64{}, Ignored: []int64{}, Rules: []string{"R1"}, Warnings: dayWarnings(date, in.Holidays)}
		d.Holiday = in.Holidays[date]
		d.Meal = Lunch
		if m := in.Meals[date]; m.Valid() {
			d.Meal = m
		}
		if len(rs) > 1 {
			d.Warnings = append(d.Warnings, "several_receipts")
		}
		if in.Combined[date] && len(rs) > 1 {
			d.Combined = true
			for _, r := range rs {
				d.ReceiptIDs = append(d.ReceiptIDs, r.ID)
				d.Total += r.Total
			}
		} else {
			d.ReceiptIDs = append(d.ReceiptIDs, rs[0].ID)
			d.Total = rs[0].Total
			for _, r := range rs[1:] {
				d.Ignored = append(d.Ignored, r.ID)
			}
		}
		days = append(days, d)
	}
	slices.SortFunc(days, func(a, b Day) int { return cmp.Compare(a.Date, b.Date) })
	return days
}

func dayWarnings(date string, holidays map[string]string) []string {
	w := []string{}
	if t, err := time.Parse(time.DateOnly, date); err == nil && (t.Weekday() == time.Saturday || t.Weekday() == time.Sunday) {
		w = append(w, "weekend")
	}
	if _, ok := holidays[date]; ok {
		w = append(w, "holiday")
	}
	return w
}

// countBest marks the limit days with the highest totals as counted (R4);
// ties go to the earlier date.
func countBest(days []Day, limit int) {
	order := make([]int, len(days))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(days[b].Total, days[a].Total), cmp.Compare(days[a].Date, days[b].Date))
	})
	for rank, i := range order {
		if rank < limit {
			days[i].Counted = true
			days[i].Rules = append(days[i].Rules, "R4")
		} else {
			days[i].Warnings = append(days[i].Warnings, "over_limit")
		}
	}
}

// split computes allowance, tax-free and taxable part of a counted day.
func split(d *Day, model TaxModel, v rates.Values) {
	value := benefitValue(d.Meal, v)
	switch model {
	case ModelTaxFree:
		// R8: the employee's own share covers the benefit value as far as it goes.
		d.Allowance = min(d.Total, v.TopUp)
		own := d.Total - d.Allowance
		d.Taxable = min(d.Allowance, max(0, value-own))
		d.Rules = append(d.Rules, "R2", "R8")
	default:
		// R6/R7: up to the benefit value taxable, the top-up above it tax-free.
		d.Allowance = min(d.Total, value+v.TopUp)
		d.Taxable = min(d.Allowance, value)
		rule := "R6"
		if model == ModelIndividual {
			rule = "R7"
		}
		d.Rules = append(d.Rules, "R2", rule)
	}
	d.TaxFree = d.Allowance - d.Taxable
}

func sum(days []Day, model TaxModel, churchBP int64) Totals {
	var t Totals
	for _, d := range days {
		if !d.Counted {
			continue
		}
		t.Days++
		t.Receipts += d.Total
		t.Allowance += d.Allowance
		t.TaxFree += d.TaxFree
		t.Taxable += d.Taxable
	}
	if model == ModelFlat {
		t.FlatTax = money.FloorPercent(t.Taxable, flatTaxBP)
		t.Soli = money.FloorPercent(t.FlatTax, soliBP)
		t.ChurchTax = money.FloorPercent(t.FlatTax, churchBP)
		t.CompanyTax = t.FlatTax + t.Soli + t.ChurchTax
	}
	return t
}
