package holidays

import "testing"

func TestEaster(t *testing.T) {
	for year, want := range map[int]string{2024: "2024-03-31", 2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := Easter(year).Format("2006-01-02"); got != want {
			t.Errorf("Easter(%d) = %s, want %s", year, got, want)
		}
	}
}

func TestBadenWuerttemberg2026(t *testing.T) {
	got := Year("BW", 2026)
	want := map[string]string{
		"2026-01-01": "Neujahr", "2026-01-06": "Heilige Drei Könige", "2026-04-03": "Karfreitag",
		"2026-04-06": "Ostermontag", "2026-05-01": "Tag der Arbeit", "2026-05-14": "Christi Himmelfahrt",
		"2026-05-25": "Pfingstmontag", "2026-06-04": "Fronleichnam", "2026-10-03": "Tag der Deutschen Einheit",
		"2026-11-01": "Allerheiligen", "2026-12-25": "1. Weihnachtstag", "2026-12-26": "2. Weihnachtstag",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d holidays, want %d: %v", len(got), len(want), got)
	}
	for d, name := range want {
		if got[d] != name {
			t.Errorf("%s: got %q, want %q", d, got[d], name)
		}
	}
}

func TestStateSpecific(t *testing.T) {
	if Year("SN", 2026)["2026-11-18"] != "Buß- und Bettag" {
		t.Error("Saxony 2026: Buß- und Bettag is 18 November")
	}
	if _, ok := Year("BW", 2026)["2026-10-31"]; ok {
		t.Error("Reformationstag is no holiday in Baden-Württemberg")
	}
	if _, ok := Year("BW", 2017)["2017-10-31"]; !ok {
		t.Error("2017 (500 years of Reformation) was a nationwide holiday")
	}
}
