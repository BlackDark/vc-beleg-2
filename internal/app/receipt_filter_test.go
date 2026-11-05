package app

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/money"
)

func TestListReceiptsFilters(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	// id -> state set directly; the reader is not under test here.
	type spec struct {
		extraction, source string
		check              bool // checked with date and total, so no warnings
	}
	specs := []spec{
		{"done", "upload", true},    // clean, analyzed
		{"done", "mail", false},     // analyzed, undated and no total: warnings
		{"failed", "mail", true},    // failed, clean
		{"pending", "folder", true}, // queued, clean
		{"off", "upload", false},    // not read, warnings
		{"off", "data", true},       // data receipt: reading off, clean, dated
	}
	ids := make([]int64, len(specs))
	for i, s := range specs {
		r := must(a.Import(ctx, u, "upload", "r.png", receiptImage(uint8(150+i)), "", nil))
		ids[i] = r.ID
		if s.check {
			must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: money.Cents(500 + i)}))
		}
		if _, err := a.DB.ExecContext(ctx, `UPDATE receipts SET extraction = ?, extracting = 0, source = ? WHERE id = ?`, s.extraction, s.source, r.ID); err != nil {
			t.Fatal(err)
		}
	}
	idx := func(is ...int) []int64 {
		out := []int64{}
		for _, i := range is {
			out = append(out, ids[i])
		}
		slices.Sort(out)
		return out
	}
	cases := []struct {
		name string
		f    ReceiptFilter
		want []int64
	}{
		// Month views list only dated receipts (0, 2, 3, 5); 1 and 4 are undated.
		{"none", ReceiptFilter{Month: "2026-10"}, idx(0, 2, 3, 5)},
		{"done", ReceiptFilter{Month: "2026-10", Extraction: "done"}, idx(0)},
		{"failed", ReceiptFilter{Month: "2026-10", Extraction: "failed"}, idx(2)},
		{"active", ReceiptFilter{Month: "2026-10", Extraction: "active"}, idx(3)},
		{"source", ReceiptFilter{Month: "2026-10", Source: "mail"}, idx(2)},
		{"month has no undated warnings", ReceiptFilter{Month: "2026-10", Warnings: true}, idx()},
		{"source+reading+status", ReceiptFilter{Month: "2026-10", Source: "mail", Extraction: "failed", Status: "checked"}, idx(2)},
		{"no match", ReceiptFilter{Month: "2026-10", Source: "folder", Extraction: "done"}, idx()},
		{"source data", ReceiptFilter{Month: "2026-10", Source: "data"}, idx(5)},
		{"source data is never read", ReceiptFilter{Month: "2026-10", Source: "data", Extraction: "off"}, idx(5)},
		{"source data, not analyzed", ReceiptFilter{Month: "2026-10", Source: "data", Extraction: "done"}, idx()},
		{"reading off in month", ReceiptFilter{Month: "2026-10", Extraction: "off"}, idx(5)},
		{"other month", ReceiptFilter{Month: "2026-11"}, idx()},
		{"no month lists all", ReceiptFilter{}, idx(0, 1, 2, 3, 4, 5)},
		// Filters combine (AND) with the undated view, which ignores the month.
		{"undated", ReceiptFilter{Undated: true}, idx(1, 4)},
		{"undated ignores month", ReceiptFilter{Undated: true, Month: "2026-11"}, idx(1, 4)},
		{"undated+done", ReceiptFilter{Undated: true, Extraction: "done"}, idx(1)},
		{"undated+off", ReceiptFilter{Undated: true, Extraction: "off"}, idx(4)},
		{"undated+source", ReceiptFilter{Undated: true, Source: "upload"}, idx(4)},
		{"undated+warnings", ReceiptFilter{Undated: true, Warnings: true}, idx(1, 4)},
		{"undated+warnings+source+reading", ReceiptFilter{Undated: true, Warnings: true, Source: "mail", Extraction: "done"}, idx(1)},
		{"undated+checked", ReceiptFilter{Undated: true, Status: "checked"}, idx()},
		{"undated+active", ReceiptFilter{Undated: true, Extraction: "active"}, idx()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := []int64{}
			for _, r := range must(a.ListReceipts(ctx, u, c.f)) {
				got = append(got, r.ID)
			}
			slices.Sort(got)
			if fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
