// Package rates holds the official meal values (Sachbezugswerte) per year.
// They live in benefit-values.csv, never in code; a new year is a new row.
package rates

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/blackdark/vc-beleg-2/internal/money"
)

//go:embed benefit-values.csv
var benefitCSV string

// Values are the meal values of one calendar year.
type Values struct {
	Year      int         `json:"year"`
	Breakfast money.Cents `json:"breakfast"`
	Lunch     money.Cents `json:"lunch"`
	Dinner    money.Cents `json:"dinner"`
	TopUp     money.Cents `json:"topUp" doc:"tax-free amount on top of the benefit value"`
	Source    string      `json:"source"`
}

// Table maps a year to its values.
type Table map[int]Values

// Load parses the embedded table.
func Load() (Table, error) { return parse(benefitCSV) }

func parse(src string) (Table, error) {
	rows, err := csv.NewReader(strings.NewReader(src)).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "year,breakfast_cents,lunch_cents,dinner_cents,top_up_cents,source" {
		return nil, fmt.Errorf("benefit values: unexpected header")
	}
	t := Table{}
	for i, r := range rows[1:] {
		line := i + 2
		year, err := strconv.Atoi(r[0])
		if err != nil || year < 2000 || year > 2100 {
			return nil, fmt.Errorf("benefit values line %d: invalid year %q", line, r[0])
		}
		if _, dup := t[year]; dup {
			return nil, fmt.Errorf("benefit values line %d: duplicate year %d", line, year)
		}
		var cents [4]money.Cents
		for j := range cents {
			n, err := strconv.ParseInt(r[j+1], 10, 64)
			if err != nil || n <= 0 || n > 10000 {
				return nil, fmt.Errorf("benefit values line %d: invalid amount %q", line, r[j+1])
			}
			cents[j] = money.Cents(n)
		}
		if strings.TrimSpace(r[5]) == "" {
			return nil, fmt.Errorf("benefit values line %d: source missing", line)
		}
		t[year] = Values{Year: year, Breakfast: cents[0], Lunch: cents[1], Dinner: cents[2], TopUp: cents[3], Source: r[5]}
	}
	return t, nil
}
