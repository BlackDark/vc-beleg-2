package calc

import (
	"fmt"
	"slices"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/holidays"
	"github.com/blackdark/vc-beleg-2/internal/money"
	"github.com/blackdark/vc-beleg-2/internal/rates"
)

var v2026 = &rates.Values{Year: 2026, Breakfast: 237, Lunch: 457, Dinner: 457, TopUp: 310}

func receiptsOn(dates []string, total money.Cents) []Receipt {
	out := make([]Receipt, len(dates))
	for i, d := range dates {
		out[i] = Receipt{ID: int64(i + 1), Date: d, Total: total}
	}
	return out
}

func dates(month string, days ...int) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = fmt.Sprintf("%s-%02d", month, d)
	}
	return out
}

func TestOwnerCaseModelA(t *testing.T) {
	// 18 supermarket lunches over €7.67: the 15 highest count.
	rs := receiptsOn(dates("2026-10", 1, 2, 5, 6, 7, 8, 9, 12, 13, 14, 15, 16, 19, 20, 21, 22, 23, 26), 900)
	rs[17].Total = 800 // 26th is the lowest of the 18
	rs[16].Total = 800 // 23rd
	rs[15].Total = 800 // 22nd
	res := Compute(Input{Month: "2026-10", Model: ModelFlat, Values: v2026, Receipts: rs, Holidays: holidays.Year("BW", 2026)})

	want := Totals{Days: 15, Receipts: 15 * 900, Allowance: 15 * 767, TaxFree: 15 * 310, Taxable: 15 * 457,
		FlatTax: 1713, Soli: 94, ChurchTax: 0, CompanyTax: 1807}
	if res.Totals != want {
		t.Fatalf("totals = %+v\nwant     %+v", res.Totals, want)
	}
	for _, d := range res.Days {
		over := d.Date == "2026-10-22" || d.Date == "2026-10-23" || d.Date == "2026-10-26"
		if d.Counted == over {
			t.Errorf("%s counted = %v", d.Date, d.Counted)
		}
		if over && !slices.Contains(d.Warnings, "over_limit") {
			t.Errorf("%s: missing over_limit warning", d.Date)
		}
	}
	if len(res.Blockers) != 0 {
		t.Errorf("blockers = %v", res.Blockers)
	}
}

func TestReceiptBelowCap(t *testing.T) {
	res := Compute(Input{Month: "2026-10", Model: ModelFlat, Values: v2026, Receipts: []Receipt{
		{ID: 1, Date: "2026-10-05", Total: 600}, // between benefit value and cap
		{ID: 2, Date: "2026-10-06", Total: 300}, // below the benefit value
	}})
	d := res.Days
	if d[0].Allowance != 600 || d[0].Taxable != 457 || d[0].TaxFree != 143 {
		t.Errorf("€6.00: %+v", d[0])
	}
	if d[1].Allowance != 300 || d[1].Taxable != 300 || d[1].TaxFree != 0 {
		t.Errorf("€3.00: %+v", d[1])
	}
}

func TestModelB(t *testing.T) {
	res := Compute(Input{Month: "2026-10", Model: ModelIndividual, Values: v2026, Receipts: []Receipt{{ID: 1, Date: "2026-10-05", Total: 1000}}})
	if res.Totals.Taxable != 457 || res.Totals.FlatTax != 0 || res.Totals.CompanyTax != 0 {
		t.Errorf("totals = %+v", res.Totals)
	}
	if !slices.Contains(res.Days[0].Rules, "R7") {
		t.Errorf("rules = %v", res.Days[0].Rules)
	}
}

func TestModelC(t *testing.T) {
	res := Compute(Input{Month: "2026-10", Model: ModelTaxFree, Values: v2026, Receipts: []Receipt{
		{ID: 1, Date: "2026-10-05", Total: 900}, // own share 5.90 ≥ 4.57: tax-free
		{ID: 2, Date: "2026-10-06", Total: 500}, // own share 1.90: 2.67 taxable
		{ID: 3, Date: "2026-10-07", Total: 200}, // allowance 2.00, own share 0
	}})
	d := res.Days
	if d[0].Allowance != 310 || d[0].Taxable != 0 {
		t.Errorf("€9.00: %+v", d[0])
	}
	if d[1].Allowance != 310 || d[1].Taxable != 267 || d[1].TaxFree != 43 {
		t.Errorf("€5.00: %+v", d[1])
	}
	if d[2].Allowance != 200 || d[2].Taxable != 200 {
		t.Errorf("€2.00: %+v", d[2])
	}
	if res.Totals.FlatTax != 0 {
		t.Errorf("model C has no flat tax: %+v", res.Totals)
	}
}

func TestSameDayReceipts(t *testing.T) {
	rs := []Receipt{{ID: 1, Date: "2026-10-05", Total: 320}, {ID: 2, Date: "2026-10-05", Total: 510}}
	single := Compute(Input{Month: "2026-10", Model: ModelFlat, Values: v2026, Receipts: rs})
	d := single.Days[0]
	if d.Total != 510 || !slices.Equal(d.ReceiptIDs, []int64{2}) || !slices.Equal(d.Ignored, []int64{1}) || !slices.Contains(d.Warnings, "several_receipts") {
		t.Errorf("highest only: %+v", d)
	}
	combined := Compute(Input{Month: "2026-10", Model: ModelFlat, Values: v2026, Receipts: rs, Combined: map[string]bool{"2026-10-05": true}})
	d = combined.Days[0]
	if d.Total != 830 || d.Allowance != 767 || len(d.ReceiptIDs) != 2 || len(d.Ignored) != 0 || !d.Combined {
		t.Errorf("combined: %+v", d)
	}
}

func TestWarningsAndScope(t *testing.T) {
	res := Compute(Input{Month: "2026-10", Model: ModelFlat, Values: v2026, Holidays: holidays.Year("BW", 2026), Receipts: []Receipt{
		{ID: 1, Date: "2026-10-03", Total: 800}, // Saturday and Tag der Deutschen Einheit
		{ID: 2, Date: "2026-09-30", Total: 800}, // previous month
	}})
	if len(res.Days) != 1 {
		t.Fatalf("days = %+v", res.Days)
	}
	d := res.Days[0]
	if !slices.Equal(d.Warnings, []string{"weekend", "holiday"}) || d.Holiday != "Tag der Deutschen Einheit" || !d.Counted {
		t.Errorf("day = %+v", d)
	}
}

func TestTieGoesToEarlierDate(t *testing.T) {
	rs := receiptsOn(dates("2026-10", 1, 2, 5), 800)
	res := Compute(Input{Month: "2026-10", Model: ModelFlat, DayLimit: 2, Values: v2026, Receipts: rs})
	if !res.Days[0].Counted || !res.Days[1].Counted || res.Days[2].Counted {
		t.Errorf("days = %+v", res.Days)
	}
}

func TestMissingValuesBlocks(t *testing.T) {
	res := Compute(Input{Month: "2027-01", Model: ModelFlat, Receipts: []Receipt{{ID: 1, Date: "2027-01-04", Total: 800}}})
	if !slices.Equal(res.Blockers, []string{"values_missing"}) || res.Totals.Allowance != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestChurchTax(t *testing.T) {
	rs := receiptsOn(dates("2026-10", 1, 2, 5, 6, 7, 8, 9, 12, 13, 14, 15, 16, 19, 20, 21), 800)
	res := Compute(Input{Month: "2026-10", Model: ModelFlat, Values: v2026, Receipts: rs, ChurchTaxBP: 550})
	if res.Totals.ChurchTax != 94 || res.Totals.CompanyTax != 1713+94+94 {
		t.Errorf("totals = %+v", res.Totals)
	}
}

func TestMeals(t *testing.T) {
	tests := []struct {
		meal                        Meal
		model                       TaxModel
		total                       money.Cents
		allowance, taxable, taxFree money.Cents
	}{
		{Lunch, ModelFlat, 900, 767, 457, 310},
		{Dinner, ModelFlat, 900, 767, 457, 310},
		{Breakfast, ModelFlat, 900, 547, 237, 310},
		{Breakfast, ModelFlat, 400, 400, 237, 163},
		{Breakfast, ModelIndividual, 200, 200, 200, 0},
		{Breakfast, ModelTaxFree, 400, 310, 147, 163}, // own share 0.90 reduces 2.37
		{Breakfast, ModelTaxFree, 600, 310, 0, 310},   // own share 2.90 covers 2.37
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s/%d", tt.meal, tt.model, tt.total), func(t *testing.T) {
			meals := map[string]Meal{}
			if tt.meal != Lunch {
				meals["2026-10-05"] = tt.meal
			}
			res := Compute(Input{Month: "2026-10", Model: tt.model, Values: v2026, Meals: meals, Receipts: []Receipt{{ID: 1, Date: "2026-10-05", Total: tt.total}}})
			d := res.Days[0]
			if d.Meal != tt.meal || d.Allowance != tt.allowance || d.Taxable != tt.taxable || d.TaxFree != tt.taxFree {
				t.Errorf("day = %+v", d)
			}
		})
	}
}
