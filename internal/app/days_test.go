package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/calc"
)

// 17 lunches in September 2026 with working-day proof: all count except
// the vacation day; the report renders the calendar.
func TestProvenWorkingDaysFlow(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	must(a.SaveSettings(ctx, u, Settings{CompanyName: "Muster GmbH", State: "BW", EmployeeName: "Max Muster", TaxModel: "A"}))
	for i, day := range []int{1, 2, 3, 4, 7, 8, 9, 10, 11, 14, 15, 16, 17, 18, 21, 22, 23} {
		r := must(a.Import(ctx, u, "upload", "bon.png", receiptImage(uint8(i*10)), "", nil))
		must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: fmt.Sprintf("2026-09-%02d", day), Total: 900, Merchant: "REWE"}))
	}
	m := must(a.GetMonth(ctx, u, "2026-09"))
	if m.Result.Totals.Days != 15 || m.Result.ProveDays {
		t.Fatalf("without proof: %+v", m.Result.Totals)
	}

	must(a.SetMark(ctx, u, "2026-09-23", calc.Vacation))
	m = must(a.SetProveDays(ctx, u, "2026-09", true))
	// September 2026 has 22 weekdays and no holiday in BW; one vacation day.
	if m.Result.Totals.Days != 16 || m.Result.Totals.WorkingDays != 21 || m.Result.DayLimit != 0 || len(m.Result.Calendar) != 30 {
		t.Fatalf("with proof: %+v, limit %d", m.Result.Totals, m.Result.DayLimit)
	}
	if _, err := a.SetMark(ctx, u, "2026-09-24", "holiday"); err == nil {
		t.Fatal("unknown mark accepted")
	}

	m = must(a.CloseMonth(ctx, u, "2026-09"))
	if m.Reports[0].Payout != 16*767 || !m.Result.ProveDays {
		t.Fatalf("closed = %+v", m.Reports)
	}
	pdf, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("no PDF")
	}
	if _, err := a.SetMark(ctx, u, "2026-09-22", calc.Sick); !errors.Is(err, ErrMonthClosed) {
		t.Fatalf("mark in closed month: %v", err)
	}
	if _, err := a.SetProveDays(ctx, u, "2026-09", false); !errors.Is(err, ErrMonthClosed) {
		t.Fatalf("proof in closed month: %v", err)
	}
}
