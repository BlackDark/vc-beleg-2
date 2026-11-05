package api

import (
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

func TestUploadReceiptData(t *testing.T) {
	s := newServer(t)
	c := s.client(t)
	file := []byte(`[{"date":"2026-10-05","total":7.85,"merchant":"REWE"},{"date":"2026-10-06","total":9,"merchant":"Bäcker"}]`)
	if st := c.do("POST", "/api/receipts/data?filename=tag.json", file, nil); st != 401 {
		t.Fatalf("anonymous upload: %d", st)
	}
	if st := c.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); st != 200 {
		t.Fatalf("setup: %d", st)
	}
	var list []app.Receipt
	if st := c.do("POST", "/api/receipts/data?filename=tag.json", file, &list); st != 201 || len(list) != 2 {
		t.Fatalf("upload: %d %+v", st, list)
	}
	if list[0].Source != "data" || list[0].Extraction != "off" || list[0].Status != "pending" || list[0].Total != 785 {
		t.Fatalf("receipt: %+v", list[0])
	}
	var e ErrorModel
	bad := []byte(`[{"date":"2026-10-05","total":7.85,"merchant":"ok"},{"date":"x","total":"1,5","merchant":"y"}]`)
	if st := c.do("POST", "/api/receipts/data?filename=bad.json", bad, &e); st != 422 || e.Code != "data.invalid" || len(e.Entries) != 2 || e.Entries[0].Index != 1 {
		t.Fatalf("invalid upload: %d %+v", st, e)
	}
	var all []app.Receipt
	if st := c.do("GET", "/api/receipts", nil, &all); st != 200 || len(all) != 2 {
		t.Fatalf("receipts after rejection: %d %d", st, len(all))
	}
	// Preview and original are served.
	if st := c.do("GET", "/api/receipts/1/preview", nil, nil); st != 200 {
		t.Fatalf("preview: %d", st)
	}
}
