package rates

import "testing"

func TestEmbeddedTable(t *testing.T) {
	tab, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	v, ok := tab[2026]
	if !ok || v.Lunch != 457 || v.Breakfast != 237 || v.TopUp != 310 {
		t.Fatalf("2026 = %+v", v)
	}
}

func TestRejectsBadRows(t *testing.T) {
	header := "year,breakfast_cents,lunch_cents,dinner_cents,top_up_cents,source\n"
	for name, body := range map[string]string{
		"bad year":   "20x6,1,1,1,1,s\n",
		"zero":       "2026,0,457,457,310,s\n",
		"no source":  "2026,237,457,457,310,\n",
		"duplicate":  "2026,237,457,457,310,s\n2026,237,457,457,310,s\n",
		"bad header": "",
	} {
		src := header + body
		if name == "bad header" {
			src = "year,lunch\n2026,457\n"
		}
		if _, err := parse(src); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
