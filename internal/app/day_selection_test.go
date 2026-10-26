package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/money"
)

func TestManualReceiptSelectionAndModes(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	high := must(a.Import(ctx, u, "upload", "high.png", receiptImage(101), "", nil))
	low := must(a.Import(ctx, u, "upload", "low.png", receiptImage(102), "", nil))
	must(a.CheckReceipt(ctx, u, high.ID, ReceiptInput{Date: "2026-10-05", Total: 900}))
	must(a.CheckReceipt(ctx, u, low.ID, ReceiptInput{Date: "2026-10-05", Total: 320}))
	m := must(a.SetSelectedReceipt(ctx, u, "2026-10-05", low.ID))
	if d := m.Result.Days[0]; d.SelectedReceiptID != low.ID || d.Total != 320 || d.Combined {
		t.Fatalf("manual choice: %+v", d)
	}
	m = must(a.GetMonth(ctx, u, "2026-10"))
	if m.Result.Days[0].SelectedReceiptID != low.ID {
		t.Fatal("selection was not retained on reload")
	}
	m = must(a.SetCombined(ctx, u, "2026-10-05", true))
	if d := m.Result.Days[0]; !d.Combined || d.Total != 1220 || d.SelectedReceiptID != 0 {
		t.Fatalf("combine clears manual: %+v", d)
	}
	m = must(a.SetSelectedReceipt(ctx, u, "2026-10-05", low.ID))
	if d := m.Result.Days[0]; d.Combined || d.Total != 320 {
		t.Fatalf("manual clears combine: %+v", d)
	}
	m = must(a.SetSelectedReceipt(ctx, u, "2026-10-05", 0))
	if d := m.Result.Days[0]; d.Combined || d.SelectedReceiptID != 0 || d.Total != 900 {
		t.Fatalf("automatic: %+v", d)
	}
	if got := must(a.ListReceipts(ctx, u, "2026-10", "checked")); len(got) != 2 {
		t.Fatal("selection discarded other receipts")
	}
}

func TestIneligibleReceiptSelectionReturnsToAutomatic(t *testing.T) {
	for _, operation := range []string{"uncheck", "move", "delete"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			a, u := newTestApp(t, nil)
			var ids []int64
			for i, total := range []money.Cents{900, 320} {
				r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(uint8(110+i)), "", nil))
				must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: total}))
				ids = append(ids, r.ID)
			}
			must(a.SetSelectedReceipt(ctx, u, "2026-10-05", ids[1]))
			switch operation {
			case "uncheck":
				must(a.UncheckReceipt(ctx, u, ids[1]))
				must(a.CheckReceipt(ctx, u, ids[1], ReceiptInput{Date: "2026-10-05", Total: 320}))
			case "move":
				must(a.UpdateReceipt(ctx, u, ids[1], ReceiptInput{Date: "2026-10-06", Total: 320}))
				must(a.UpdateReceipt(ctx, u, ids[1], ReceiptInput{Date: "2026-10-05", Total: 320}))
			case "delete":
				if err := a.DeleteReceipt(ctx, u, ids[1]); err != nil {
					t.Fatal(err)
				}
			}
			d := must(a.GetMonth(ctx, u, "2026-10")).Result.Days[0]
			if d.SelectedReceiptID != 0 || d.Total != 900 || d.Combined {
				t.Fatalf("selection restored after %s: %+v", operation, d)
			}
		})
	}
}

func TestReceiptSelectionRejectsIneligibleReceipts(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(120), "", nil))
	if _, err := a.SetSelectedReceipt(ctx, u, "2026-10-05", r.ID); err == nil {
		t.Fatal("selected pending receipt")
	}
	must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: 320}))
	for _, date := range []string{"", "bad", "2026-10-06"} {
		if _, err := a.SetSelectedReceipt(ctx, u, date, r.ID); err == nil {
			t.Fatalf("selected receipt for %q", date)
		}
	}
	other := &User{ID: u.ID + 1, EmployeeID: u.EmployeeID + 1}
	if _, err := a.SetSelectedReceipt(ctx, other, "2026-10-05", r.ID); err == nil {
		t.Fatal("selected another employee's receipt")
	}
	for _, id := range []int64{-1, r.ID + 1} {
		if _, err := a.SetSelectedReceipt(ctx, u, "2026-10-05", id); err == nil {
			t.Fatalf("selected invalid ID %d", id)
		}
	}
}

func TestClosedMonthKeepsManualReceiptSelection(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	var ids []int64
	for i, total := range []money.Cents{900, 320} {
		r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(uint8(130+i)), "", nil))
		must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: total}))
		ids = append(ids, r.ID)
	}
	must(a.SetSelectedReceipt(ctx, u, "2026-10-05", ids[1]))
	m := must(a.CloseMonth(ctx, u, "2026-10"))
	if d := m.Result.Days[0]; d.SelectedReceiptID != ids[1] || d.Allowance != 320 {
		t.Fatalf("closed: %+v", d)
	}
	pdf, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	for _, id := range []int64{0, ids[0]} {
		if _, err := a.SetSelectedReceipt(ctx, u, "2026-10-05", id); !errors.Is(err, ErrMonthClosed) {
			t.Fatalf("changed closed selection: %v", err)
		}
	}
	if _, err := a.SetCombined(ctx, u, "2026-10-05", true); !errors.Is(err, ErrMonthClosed) {
		t.Fatalf("combined closed selection: %v", err)
	}
	must(a.ReopenMonth(ctx, u, "2026-10", "choose other receipt"))
	must(a.SetSelectedReceipt(ctx, u, "2026-10-05", ids[0]))
	must(a.CloseMonth(ctx, u, "2026-10"))
	old, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	if !bytes.Equal(pdf, old) {
		t.Fatal("prior selected report changed")
	}
}
