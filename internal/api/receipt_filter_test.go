package api

import (
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

func TestListReceiptsFilterParameters(t *testing.T) {
	s := newServer(t)
	c := s.client(t)
	if st := c.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); st != 200 {
		t.Fatalf("setup: %d", st)
	}
	var r app.Receipt
	if st := c.do("POST", "/api/receipts?filename=receipt.png", pngBytes(), &r); st != 201 {
		t.Fatalf("upload: %d", st)
	}
	// Without a configured reader the fresh upload is "off", from upload, and undated (so it has warnings, and shows only with undated=true or no month).
	cases := []struct {
		query string
		want  int
	}{
		{"", 1},
		{"?reading=off", 1},
		{"?reading=active", 0},
		{"?reading=done", 0},
		{"?source=upload", 1},
		{"?source=mail", 0},
		{"?warnings=true", 1},
		{"?warnings=true&source=mail", 0},
		{"?undated=true&reading=off&source=upload&warnings=true", 1},
		{"?undated=true&reading=done", 0},
		{"?undated=true&month=2026-10&warnings=true", 1},
		{"?month=2026-10&reading=off&source=upload&warnings=true", 0}, // month views list only dated receipts
	}
	for _, c2 := range cases {
		var list []app.Receipt
		if st := c.do("GET", "/api/receipts"+c2.query, nil, &list); st != 200 || len(list) != c2.want {
			t.Errorf("%q: status %d, %d receipts, want %d", c2.query, st, len(list), c2.want)
		}
	}
	if st := c.do("GET", "/api/receipts?reading=bogus", nil, nil); st != 422 {
		t.Fatalf("invalid reading: %d", st)
	}
}
