package api

import (
	"fmt"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

func TestSelectReceiptRequiresLoginAndEligibleReceipt(t *testing.T) {
	s := newServer(t)
	c := s.client(t)
	choice := map[string]any{"date": "2026-10-05", "receiptId": 1}
	if st := c.do("POST", "/api/days/select", choice, nil); st != 401 {
		t.Fatalf("anonymous selection: %d", st)
	}
	if st := c.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); st != 200 {
		t.Fatalf("setup: %d", st)
	}
	var low int64
	for _, total := range []int{320, 900} {
		var r app.Receipt
		if st := c.do("POST", "/api/receipts?filename=receipt.png", pngBytes(), &r); st != 201 {
			t.Fatalf("upload: %d", st)
		}
		if low == 0 {
			low = r.ID
		}
		if st := c.do("POST", fmt.Sprintf("/api/receipts/%d/check", r.ID), map[string]any{"date": "2026-10-05", "total": total, "merchant": "Shop", "note": ""}, nil); st != 200 {
			t.Fatalf("check: %d", st)
		}
	}
	var m app.Month
	choice["receiptId"] = low
	if st := c.do("POST", "/api/days/select", choice, &m); st != 200 || m.Result.Totals.Allowance != 320 {
		t.Fatalf("select: %d %+v", st, m)
	}
	choice["date"] = "2026-10-06"
	if st := c.do("POST", "/api/days/select", choice, nil); st != 422 {
		t.Fatalf("wrong day: %d", st)
	}
	choice["date"], choice["receiptId"] = "2026-10-05", 0
	if st := c.do("POST", "/api/days/select", choice, &m); st != 200 || m.Result.Totals.Allowance != 767 {
		t.Fatalf("automatic: %d %+v", st, m)
	}
}
