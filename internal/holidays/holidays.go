// Package holidays lists statutory public holidays per German state
// (Bundesland). Holidays that apply only to parts of a state (Augsburger
// Friedensfest, Fronleichnam in parts of Saxony and Thuringia, Mariä
// Himmelfahrt in parts of Bavaria) are left out.
package holidays

import (
	"slices"
	"time"
)

// States are the ISO 3166-2:DE codes without the "DE-" prefix.
var States = []string{"BW", "BY", "BE", "BB", "HB", "HH", "HE", "MV", "NI", "NW", "RP", "SL", "SN", "ST", "SH", "TH"}

// ValidState reports whether s is a known state code.
func ValidState(s string) bool { return slices.Contains(States, s) }

type rule struct {
	name   string
	date   func(year int, easter time.Time) time.Time
	states []string // nil: nationwide
	from   int      // first year, 0: always
	until  int      // last year, 0: open
}

func fixed(month time.Month, day int) func(int, time.Time) time.Time {
	return func(year int, _ time.Time) time.Time { return time.Date(year, month, day, 0, 0, 0, 0, time.UTC) }
}

func easterOffset(days int) func(int, time.Time) time.Time {
	return func(_ int, e time.Time) time.Time { return e.AddDate(0, 0, days) }
}

// repentance is Buß- und Bettag: the Wednesday before 23 November.
func repentance(year int, _ time.Time) time.Time {
	d := time.Date(year, time.November, 22, 0, 0, 0, 0, time.UTC)
	for d.Weekday() != time.Wednesday {
		d = d.AddDate(0, 0, -1)
	}
	return d
}

var rules = []rule{
	{name: "Neujahr", date: fixed(time.January, 1)},
	{name: "Heilige Drei Könige", date: fixed(time.January, 6), states: []string{"BW", "BY", "ST"}},
	{name: "Internationaler Frauentag", date: fixed(time.March, 8), states: []string{"BE"}, from: 2019},
	{name: "Internationaler Frauentag", date: fixed(time.March, 8), states: []string{"MV"}, from: 2023},
	{name: "Karfreitag", date: easterOffset(-2)},
	{name: "Ostermontag", date: easterOffset(1)},
	{name: "Tag der Arbeit", date: fixed(time.May, 1)},
	{name: "Christi Himmelfahrt", date: easterOffset(39)},
	{name: "Pfingstmontag", date: easterOffset(50)},
	{name: "Fronleichnam", date: easterOffset(60), states: []string{"BW", "BY", "HE", "NW", "RP", "SL"}},
	{name: "Mariä Himmelfahrt", date: fixed(time.August, 15), states: []string{"SL"}},
	{name: "Weltkindertag", date: fixed(time.September, 20), states: []string{"TH"}, from: 2019},
	{name: "Tag der Deutschen Einheit", date: fixed(time.October, 3)},
	{name: "Reformationstag", date: fixed(time.October, 31), states: []string{"BB", "SN", "ST", "MV", "TH"}},
	{name: "Reformationstag", date: fixed(time.October, 31), states: []string{"HB", "HH", "NI", "SH"}, from: 2018},
	{name: "Reformationstag", date: fixed(time.October, 31), states: []string{"BE", "BW", "BY", "HE", "NW", "RP", "SL", "HB", "HH", "NI", "SH"}, from: 2017, until: 2017},
	{name: "Allerheiligen", date: fixed(time.November, 1), states: []string{"BW", "BY", "NW", "RP", "SL"}},
	{name: "Buß- und Bettag", date: repentance, states: []string{"SN"}},
	{name: "1. Weihnachtstag", date: fixed(time.December, 25)},
	{name: "2. Weihnachtstag", date: fixed(time.December, 26)},
}

// Easter returns Easter Sunday (Gregorian, anonymous algorithm).
func Easter(year int) time.Time {
	a := year % 19
	b, c := year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// Year returns the state's holidays of a year as date ("2006-01-02") → name.
func Year(state string, year int) map[string]string {
	easter := Easter(year)
	out := map[string]string{}
	for _, r := range rules {
		if r.from > year || (r.until != 0 && r.until < year) || (r.states != nil && !slices.Contains(r.states, state)) {
			continue
		}
		out[r.date(year, easter).Format(time.DateOnly)] = r.name
	}
	return out
}
