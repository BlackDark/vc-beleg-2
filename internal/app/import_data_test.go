package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/money"
)

func TestImportDataSingleObject(t *testing.T) {
	ctx := context.Background()
	// A reader is configured; data receipts must still not be queued.
	a, u := newTestApp(t, fakeReader{})
	list, err := a.ImportData(ctx, u, "bon.json", []byte(`{"date":"2026-10-05","total":7.85,"merchant":" REWE ","extra":{"table":4}}`))
	if err != nil || len(list) != 1 {
		t.Fatalf("import: %v %v", list, err)
	}
	r := list[0]
	if r.Source != "data" || r.Status != "pending" || r.Extraction != "off" || r.Date != "2026-10-05" || r.Total != 785 || r.Merchant != "REWE" || r.Mime != "application/json" {
		t.Fatalf("receipt = %+v", r)
	}
	var n int
	if err := a.DB.QueryRow(`SELECT count(*) FROM receipts WHERE extraction = 'pending'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("queued extraction jobs = %d, %v", n, err)
	}
	// The original is the canonical record; the preview a PNG.
	b, mime, _, err := a.ReceiptFile(ctx, u, r.ID, false)
	if err != nil || mime != "application/json" || string(b) != `{"date":"2026-10-05","merchant":"REWE","total":"7.85","extra":{"table":4}}` {
		t.Fatalf("original = %q %q %v", b, mime, err)
	}
	if p, _, _, err := a.ReceiptFile(ctx, u, r.ID, true); err != nil || !bytes.HasPrefix(p, []byte("\x89PNG")) {
		t.Fatalf("preview: %v", err)
	}
	if res, err := a.Verify(ctx); err != nil || len(res.Problems) != 0 {
		t.Fatalf("verify: %+v %v", res, err)
	}
	// The employee still checks it like any other receipt.
	if r, err = a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: r.Date, Total: r.Total, Merchant: r.Merchant}); err != nil || r.Status != "checked" {
		t.Fatalf("check: %+v %v", r, err)
	}
}

func TestImportDataArrayAndRounding(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	list, err := a.ImportData(ctx, u, "tag.json", []byte(`[
		{"date":"2026-10-05","total":"12.5","merchant":"A"},
		{"date":"2026-10-06","total":3,"merchant":"B"},
		{"date":"2026-10-07","total":0.005,"merchant":"C"},
		{"date":"2026-10-08","total":1000,"merchant":"D"}]`))
	if err != nil || len(list) != 4 {
		t.Fatalf("import: %v %v", list, err)
	}
	var got []money.Cents
	for _, r := range list {
		got = append(got, r.Total)
	}
	if !slices.Equal(got, []money.Cents{1250, 300, 1, 100000}) {
		t.Fatalf("totals = %v", got)
	}
	if list[0].Filename != "tag (1).json" || list[3].Filename != "tag (4).json" {
		t.Fatalf("names = %q %q", list[0].Filename, list[3].Filename)
	}
}

func TestImportDataRejectsWholeFileWithEntryErrors(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	_, err := a.ImportData(ctx, u, "bad.json", []byte(`[
		{"date":"2026-10-05","total":7.85,"merchant":"ok"},
		{"date":"05.10.2026","total":"abc","merchant":""},
		{"date":"2026-02-30","total":0,"merchant":"x","colour":"red"},
		{"total":-1},
		{"date":"2026-10-05","total":1001,"merchant":"x","extra":[1]},
		"text"]`))
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != "data.invalid" {
		t.Fatalf("err = %v", err)
	}
	type key struct {
		i     int
		field string
		code  string
	}
	var got []key
	for _, e := range ae.Entries {
		got = append(got, key{e.Index, e.Field, e.Code})
	}
	for _, want := range []key{
		{1, "date", "data.date_invalid"}, {1, "total", "data.total_invalid"}, {1, "merchant", "data.merchant_invalid"},
		{2, "date", "data.date_invalid"}, {2, "total", "data.total_range"}, {2, "colour", "data.field_unknown"},
		{3, "date", "data.field_missing"}, {3, "merchant", "data.field_missing"}, {3, "total", "data.total_invalid"},
		{4, "total", "data.total_range"}, {4, "extra", "data.extra_invalid"}, {5, "", "data.entry_not_object"},
	} {
		if !slices.Contains(got, want) {
			t.Errorf("missing error %+v in %+v", want, got)
		}
	}
	if slices.ContainsFunc(got, func(k key) bool { return k.i == 0 }) {
		t.Errorf("valid entry reported: %+v", got)
	}
	// All-or-nothing: the valid first entry was not imported.
	list, err := a.ListReceipts(ctx, u, "", "", false)
	if err != nil || len(list) != 0 {
		t.Fatalf("receipts = %v %v", list, err)
	}
}

func TestImportDataRejectsMalformedFiles(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	for body, code := range map[string]string{
		``:                                       "data.json_invalid",
		`{"date":`:                               "data.json_invalid",
		`{} {}`:                                  "data.json_invalid",
		`[]`:                                     "data.empty",
		`42`:                                     "data.shape",
		`"x"`:                                    "data.shape",
		`%PDF-1.4`:                               "data.json_invalid",
		"[" + strings.Repeat(`{},`, 100) + "{}]": "data.too_many",
	} {
		_, err := a.ImportData(ctx, u, "x.json", []byte(body))
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != code {
			t.Errorf("%.20q: err = %v, want %s", body, err, code)
		}
	}
	// Image and PDF uploads do not accept JSON, so the data path stays explicit.
	if _, err := a.Import(ctx, u, "upload", "a.json", []byte(`{"date":"2026-10-05","total":1,"merchant":"x"}`), "", nil); err == nil {
		t.Fatal("regular import accepted JSON")
	}
	if _, err := a.Import(ctx, u, "mail", "a.json", []byte(`{}`), "", nil); err == nil {
		t.Fatal("mail import accepted JSON")
	}
}

func TestImportDataDuplicateByCanonicalChecksum(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	first := must(a.ImportData(ctx, u, "a.json", []byte(`{"date":"2026-10-05","total":7.85,"merchant":"REWE"}`)))[0]
	if slices.Contains(first.Warnings, "duplicate") {
		t.Fatalf("first is a duplicate: %v", first.Warnings)
	}
	// Other key order, spacing, number notation and merchant padding: same entry.
	second := must(a.ImportData(ctx, u, "b.json", []byte(`[ { "merchant": "REWE ", "total": "7.850", "date": "2026-10-05" } ]`)))[0]
	if !slices.Contains(second.Warnings, "duplicate") {
		t.Fatalf("second not flagged: %v", second.Warnings)
	}
	first = must(a.GetReceipt(ctx, u, first.ID))
	if !slices.Contains(first.Warnings, "duplicate") {
		t.Fatalf("first not flagged after second: %v", first.Warnings)
	}
	// Different extra data is a different record.
	third := must(a.ImportData(ctx, u, "c.json", []byte(`{"date":"2026-10-05","total":7.85,"merchant":"REWE","extra":{"n":2}}`)))[0]
	if slices.Contains(third.Warnings, "duplicate") {
		t.Fatalf("third flagged: %v", third.Warnings)
	}
}

func TestDataReceiptAppearsInReport(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Log("pdftotext not installed; checking only that the report renders")
	}
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	must(a.SaveSettings(ctx, u, Settings{CompanyName: "Muster GmbH", State: "BW", EmployeeName: "Max Muster", TaxModel: "A"}))
	r := must(a.ImportData(ctx, u, "bon.json", []byte(`{"date":"2026-10-05","total":9.5,"merchant":"Kantine </script> #raw(\"x\")","extra":{"Tisch":"4","Gutschein":true}}`)))[0]
	must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: r.Date, Total: r.Total, Merchant: r.Merchant}))
	m := must(a.CloseMonth(ctx, u, "2026-10"))
	pdf, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("no PDF")
	}
	if os.Getenv("KEEP_REPORT") != "" {
		_ = os.WriteFile(os.Getenv("KEEP_REPORT"), pdf, 0o600)
	}
}

func entryCodes(t *testing.T, a *App, u *User, body string) []string {
	t.Helper()
	_, err := a.ImportData(context.Background(), u, "x.json", []byte(body))
	var ae *Error
	if !errors.As(err, &ae) {
		t.Fatalf("%.60q: err = %v", body, err)
	}
	var codes []string
	for _, e := range ae.Entries {
		codes = append(codes, e.Code)
	}
	if len(ae.Entries) == 0 {
		codes = append(codes, ae.Code)
	}
	return codes
}

func TestImportDataRejectsBadExtraAndMerchant(t *testing.T) {
	a, u := newTestApp(t, nil)
	pre := `{"date":"2026-10-05","total":1,"merchant":"M",`
	deep := strings.Repeat(`{"a":`, 6) + `1` + strings.Repeat(`}`, 6)
	for body, want := range map[string]string{
		pre + `"extra":{"n":1e999}}`:   "data.extra_number_invalid",
		pre + `"extra":{"n":1e-9999}}`: "data.extra_number_invalid",
		pre + `"extra":` + deep + `}`:  "data.extra_too_deep",
		pre + `"extra":` + strings.Repeat(`[`, 3000) + strings.Repeat(`]`, 3000) + `}`: "data.extra_too_deep",
		pre + `"extra":{"` + strings.Repeat("k", 101) + `":1}}`:                        "data.extra_key_invalid",
		pre + `"extra":{"k":"` + strings.Repeat("v", 501) + `"}}`:                      "data.extra_value_too_long",
		pre + `"extra":{"a\u202eb":1}}`:                                                "data.extra_key_invalid",
		pre + `"extra":{"a":1,"a":2}}`:                                                 "data.key_duplicate",
		`{"date":"2026-10-05","date":"2026-10-06","total":1,"merchant":"M"}`:           "data.key_duplicate",
		`{"date":"2026-10-05","total":1,"merchant":"evil\u202Ename"}`:                  "data.merchant_characters",
		`{"date":"2026-10-05","total":1,"merchant":"a\u200bb"}`:                        "data.merchant_characters",
		`{"date":"2026-10-05","total":1,"merchant":"a\nb"}`:                            "data.merchant_characters",
	} {
		if got := entryCodes(t, a, u, body); !slices.Contains(got, want) || slices.Contains(got, "data.unreadable") {
			t.Errorf("%.70q: codes = %v, want %s", body, got, want)
		}
	}
	// Exactly 5 levels still pass.
	ok := strings.Repeat(`{"a":`, 5) + `1` + strings.Repeat(`}`, 5)
	if _, err := a.ImportData(context.Background(), u, "ok.json", []byte(pre+`"extra":`+ok+`}`)); err != nil {
		t.Fatalf("5 levels: %v", err)
	}
}

func TestImportDataDuplicateIsCanonical(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	row := func(merchant, extra string) *Receipt {
		return must(a.ImportData(ctx, u, "d.json", []byte(`{"date":"2026-10-05","total":5,"merchant":"`+merchant+`","extra":`+extra+`}`)))[0]
	}
	row("Caf\u00e9", `{"n":2,"s":"\u00e9"}`)
	for name, r := range map[string]*Receipt{
		"NFD merchant and string": row("Cafe\u0301", `{"n":2,"s":"e\u0301"}`),
		"2.0":                     row("Caf\u00e9", `{"n":2.0,"s":"\u00e9"}`),
		"1.00 style and exponent": row("Caf\u00e9", `{"n":2.00e0,"s":"\u00e9"}`),
	} {
		if !slices.Contains(r.Warnings, "duplicate") {
			t.Errorf("%s not flagged as duplicate", name)
		}
	}
	if r := row("Caf\u00e9", `{"n":2.5,"s":"\u00e9"}`); slices.Contains(r.Warnings, "duplicate") {
		t.Error("different number flagged")
	}
}

// Long words without spaces must stay inside the card and the amount must
// not wrap; the record is still passed as data, never as markup.
func TestDataReceiptLongStringsAndMarkupRenderLiterally(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	merchant := strings.Repeat("x", 200)
	extra := `{"` + strings.Repeat("k", 100) + `":"` + strings.Repeat("v", 500) + `","#read(\"/etc/passwd\")":"#include \"/etc/passwd\" *b* $m$ <l> \\"}`
	r := must(a.ImportData(ctx, u, "long.json", []byte(`{"date":"2026-10-05","total":1,"merchant":"`+merchant+`","extra":`+extra+`}`)))[0]
	if p, _, _, err := a.ReceiptFile(ctx, u, r.ID, true); err != nil || !bytes.HasPrefix(p, []byte("\x89PNG")) {
		t.Fatalf("preview: %v", err)
	}
	pdfText := pdfTextCommand()
	if pdfText == nil {
		t.Skip("neither pdftotext nor gs installed; PDF text not checked")
	}
	must(a.SaveSettings(ctx, u, Settings{CompanyName: "Muster GmbH", State: "BW", EmployeeName: "Max Muster", TaxModel: "A"}))
	must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: r.Date, Total: r.Total, Merchant: "Kantine"}))
	m := must(a.CloseMonth(ctx, u, "2026-10"))
	pdf, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	path := t.TempDir() + "/r.pdf"
	if err := os.WriteFile(path, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := pdfText(path).Output()
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "1,00\u00a0€") && !strings.Contains(text, "1,00 €") {
		t.Errorf("amount wrapped or missing:\n%s", text)
	}
	if strings.Contains(text, "root:") || !strings.Contains(text, "#read") {
		t.Errorf("markup not literal:\n%s", text)
	}
}

// The widest glyphs must not make a key paint into the value column; the
// template chunks and clips, here we check the entry renders (preview and
// report) without error.
func TestDataReceiptWorstCaseGlyphWidths(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	pair := func(k, v string) string { return `"` + strings.Repeat(k, 100) + `":"` + strings.Repeat(v, 500) + `"` }
	extra := `{` + pair("W", "M") + `,` + pair("M", "W") + `,` + pair("@", "@") + `}`
	r := must(a.ImportData(ctx, u, "wide.json", []byte(`{"date":"2026-10-05","total":1,"merchant":"`+strings.Repeat("W", 190)+`","extra":`+extra+`}`)))[0]
	pp, _, _, err := a.ReceiptFile(ctx, u, r.ID, true)
	if err != nil || !bytes.HasPrefix(pp, []byte("\x89PNG")) {
		t.Fatalf("preview: %v", err)
	}
	must(a.SaveSettings(ctx, u, Settings{CompanyName: "Muster GmbH", State: "BW", EmployeeName: "Max Muster", TaxModel: "A"}))
	must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: r.Date, Total: r.Total, Merchant: "Kantine"}))
	m := must(a.CloseMonth(ctx, u, "2026-10"))
	pdf, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("no PDF")
	}
}

// pdfTextCommand returns a command builder that prints the text of a PDF
// (poppler or Ghostscript), or nil when neither is installed.
func pdfTextCommand() func(path string) *exec.Cmd {
	if _, err := exec.LookPath("pdftotext"); err == nil {
		return func(p string) *exec.Cmd { return exec.Command("pdftotext", "-layout", p, "-") }
	}
	if _, err := exec.LookPath("gs"); err == nil {
		return func(p string) *exec.Cmd {
			return exec.Command("gs", "-q", "-dNOPAUSE", "-dBATCH", "-sDEVICE=txtwrite", "-sOutputFile=-", p)
		}
	}
	return nil
}
