package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/db"
	"github.com/blackdark/vc-beleg-2/internal/export"
	"github.com/blackdark/vc-beleg-2/internal/money"
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

type fakeReader struct{ ex Extraction }

func (f fakeReader) Extract(context.Context, string, []byte) (Extraction, error) { return f.ex, nil }

// newTestApp opens a fresh app with real SQLite and Typst at a fixed time.
func newTestApp(t *testing.T, ex Extractor) (*App, *User) {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fonts := filepath.Join(dir, "fonts")
	if err := export.ExtractFonts(fonts); err != nil {
		t.Fatal(err)
	}
	typst := os.Getenv("MAHLZEIT_TYPST")
	if typst == "" {
		typst = "typst"
	}
	a, err := New(conn, Config{DataDir: dir, Typst: typst, FontDir: fonts, MaxUploadBytes: 10 << 20, MaxPixels: 40_000_000, MaxPDFPages: 10}, slog.New(slog.NewTextHandler(io.Discard, nil)), ex)
	if err != nil {
		t.Fatal(err)
	}
	a.now = func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }
	u := must(a.Setup(ctx, "tok", "tok", "Owner", "Max Muster", "a-long-password"))
	return a, u
}

// receiptImage returns a distinct small PNG.
func receiptImage(shade uint8) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 120, 200))
	for x := range 120 {
		for y := range 200 {
			img.Set(x, y, color.RGBA{shade, 200, 200, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func TestSetupIsSingleUse(t *testing.T) {
	a, _ := newTestApp(t, nil)
	_, err := a.Setup(context.Background(), "tok", "tok", "second", "Second", "a-long-password")
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != "setup.done" {
		t.Fatalf("second setup: %v", err)
	}
}

func TestMonthFlow(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	must(a.SaveSettings(ctx, u, Settings{CompanyName: "Muster GmbH", State: "BW", EmployeeName: "Max Muster", TaxModel: "A", LegalBasis: "Gesellschafterbeschluss vom 02.01.2026"}))

	var ids []int64
	for i, day := range []string{"2026-10-05", "2026-10-06", "2026-10-06", "2026-10-07"} {
		r := must(a.Import(ctx, u, "upload", "bon.png", receiptImage(uint8(i*40)), "", nil))
		if r.Status != "pending" || r.Extraction != "off" {
			t.Fatalf("import: %+v", r)
		}
		ids = append(ids, r.ID)
		total := int64(850)
		if i == 2 {
			total = 320
		}
		if i < 3 {
			must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: day, Total: money.Cents(total), Merchant: "REWE"}))
		} else {
			must(a.UpdateReceipt(ctx, u, r.ID, ReceiptInput{Date: day, Total: 900}))
		}
	}

	m := must(a.GetMonth(ctx, u, "2026-10"))
	if m.Pending != 1 || !contains(m.Blockers, "pending_receipts") {
		t.Fatalf("pending receipt must block closing: %+v", m)
	}
	if _, err := a.CloseMonth(ctx, u, "2026-10"); err == nil {
		t.Fatal("closed with a pending receipt")
	}
	must(a.CheckReceipt(ctx, u, ids[3], ReceiptInput{Date: "2026-10-07", Total: 900, Merchant: "Bäcker"}))

	// Two receipts on the 6th: the higher counts until the day is combined.
	m = must(a.GetMonth(ctx, u, "2026-10"))
	if m.Result.Totals.Days != 3 || m.Result.Totals.Allowance != 3*767 {
		t.Fatalf("totals = %+v", m.Result.Totals)
	}
	m = must(a.SetCombined(ctx, u, "2026-10-06", true))
	if d := m.Result.Days[1]; !d.Combined || d.Total != 1170 {
		t.Fatalf("combined day = %+v", d)
	}

	m = must(a.CloseMonth(ctx, u, "2026-10"))
	if m.Status != "closed" || len(m.Reports) != 1 || m.Reports[0].Payout != 3*767 {
		t.Fatalf("closed = %+v", m)
	}
	pdf, name := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || name != "Essenszuschuss-2026-10-v1.pdf" {
		t.Fatalf("report %q", name)
	}

	// Closed months are frozen.
	if _, err := a.UncheckReceipt(ctx, u, ids[0]); !errors.Is(err, ErrMonthClosed) {
		t.Fatalf("uncheck in closed month: %v", err)
	}
	late := must(a.Import(ctx, u, "mail", "late.png", receiptImage(250), "", &MailMeta{MessageID: "<m1@x>", From: "me@example.com"}))
	late = must(a.UpdateReceipt(ctx, u, late.ID, ReceiptInput{Date: "2026-10-08", Total: 700}))
	if !contains(late.Warnings, "month_closed") {
		t.Fatalf("late receipt warnings = %v", late.Warnings)
	}
	if _, err := a.CheckReceipt(ctx, u, late.ID, ReceiptInput{Date: "2026-10-08", Total: 700}); !errors.Is(err, ErrMonthClosed) {
		t.Fatalf("check into closed month: %v", err)
	}

	// Reopen, add the late receipt, close again: v1 stays byte-identical.
	must(a.ReopenMonth(ctx, u, "2026-10", "Beleg vom 8. nachgereicht"))
	must(a.CheckReceipt(ctx, u, late.ID, ReceiptInput{Date: "2026-10-08", Total: 700}))
	m = must(a.CloseMonth(ctx, u, "2026-10"))
	if len(m.Reports) != 2 || m.Reports[0].Version != 2 || m.Reports[0].ReopenReason != "Beleg vom 8. nachgereicht" || m.Reports[0].Payout != 3*767+700 {
		t.Fatalf("v2 = %+v", m.Reports)
	}
	v1, _ := must2(a.ReportFile(ctx, u, m.Reports[1].ID))
	if !bytes.Equal(v1, pdf) {
		t.Fatal("v1 changed")
	}

	m = must(a.SetPaid(ctx, u, "2026-10", "2026-11-01"))
	if m.Status != "paid" || m.PaidOn != "2026-11-01" {
		t.Fatalf("paid = %+v", m)
	}
	if _, err := a.ReopenMonth(ctx, u, "2026-10", "x"); err == nil {
		t.Fatal("reopened a paid month")
	}
	if r := must(a.Verify(ctx)); len(r.Problems) != 0 || r.Reports != 2 || r.Receipts != 5 {
		t.Fatalf("verify = %+v", r)
	}
}

func TestReceiptRules(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	img := receiptImage(10)
	r1 := must(a.Import(ctx, u, "upload", "a.png", img, "", nil))
	r2 := must(a.Import(ctx, u, "upload", "b.png", img, "", nil))
	if !contains(r2.Warnings, "duplicate") || !contains(r2.Warnings, "date_missing") {
		t.Fatalf("warnings = %v", r2.Warnings)
	}
	if _, err := a.CheckReceipt(ctx, u, r1.ID, ReceiptInput{Total: 500}); err == nil {
		t.Fatal("checked without date")
	}
	must(a.CheckReceipt(ctx, u, r1.ID, ReceiptInput{Date: "2026-10-05", Total: 500}))
	if err := a.DeleteReceipt(ctx, u, r2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Import(ctx, u, "upload", "x.txt", []byte("hello"), "", nil); err == nil {
		t.Fatal("accepted a text file")
	}
	got := must(a.ListReceipts(ctx, u, "2026-10", "checked"))
	if len(got) != 1 || got[0].Number != "B-2026-0001" {
		t.Fatalf("list = %+v", got)
	}
}

func TestExtractionFillsEmptyFields(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, fakeReader{Extraction{Date: "2026-10-09", Total: 789, Merchant: "Lidl", Uncertain: []string{"total"}}})
	r := must(a.Import(ctx, u, "upload", "a.png", receiptImage(1), "", nil))
	if r.Extraction != "pending" {
		t.Fatalf("extraction = %s", r.Extraction)
	}
	must(a.UpdateReceipt(ctx, u, r.ID, ReceiptInput{Merchant: "Lidl Filiale 12"}))
	a.extractOne(ctx, <-a.extractQueue)
	r = must(a.GetReceipt(ctx, u, r.ID))
	if r.Extraction != "done" || r.Date != "2026-10-09" || r.Total != 789 || r.Merchant != "Lidl Filiale 12" || !contains(r.Extracted.Uncertain, "total") {
		t.Fatalf("receipt = %+v", r)
	}
}

func must2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		panic(err)
	}
	return a, b
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
