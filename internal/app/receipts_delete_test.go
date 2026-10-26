package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/db"
	"github.com/blackdark/vc-beleg-2/internal/money"
	"github.com/pressly/goose/v3"
)

func TestDeleteCheckedReceiptNeverArchived(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	r := must(a.Import(ctx, u, "upload", "wrong.png", receiptImage(70), "", nil))
	r = must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: 500}))
	if err := a.DeleteReceipt(ctx, u, r.ID); err != nil {
		t.Fatalf("delete checked receipt never archived: %v", err)
	}
	if _, err := a.GetReceipt(ctx, u, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted receipt: %v", err)
	}
	next := must(a.Import(ctx, u, "upload", "next.png", receiptImage(71), "", nil))
	next = must(a.CheckReceipt(ctx, u, next.ID, ReceiptInput{Date: "2026-10-06", Total: 600}))
	if next.Number != "B-2026-0002" {
		t.Fatalf("reused receipt number: %s", next.Number)
	}
}

// Every receipt in the snapshot is evidence, even when another receipt of
// the same day supplied the allowance and only that original was appended.
func TestArchivedReceiptsStayProtectedAfterReopening(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	var ids []int64
	for i, total := range []money.Cents{500, 900} {
		r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(uint8(80+i)), "", nil))
		must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: total}))
		ids = append(ids, r.ID)
	}
	m := must(a.CloseMonth(ctx, u, "2026-10"))
	pdf, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	for _, id := range ids {
		r := must(a.GetReceipt(ctx, u, id))
		if !r.Archived {
			t.Fatal("archived receipt not identified")
		}
		var ae *Error
		if err := a.DeleteReceipt(ctx, u, id); !errors.As(err, &ae) || ae.Code != "receipt.archived" {
			t.Fatalf("delete archived receipt: %v", err)
		}
	}
	must(a.ReopenMonth(ctx, u, "2026-10", "correct receipt"))
	must(a.UncheckReceipt(ctx, u, ids[0]))
	must(a.UpdateReceipt(ctx, u, ids[0], ReceiptInput{Date: "2026-09-03", Total: 500}))
	must(a.CheckReceipt(ctx, u, ids[1], ReceiptInput{Date: "2026-09-04", Total: 900}))
	for _, id := range ids {
		if err := a.DeleteReceipt(ctx, u, id); err == nil {
			t.Fatal("deleted archived evidence after reopening and moving date")
		}
		if _, _, _, err := a.ReceiptFile(ctx, u, id, false); err != nil {
			t.Fatalf("archived original missing: %v", err)
		}
	}
	old, _ := must2(a.ReportFile(ctx, u, m.Reports[0].ID))
	if !bytes.Equal(pdf, old) {
		t.Fatal("archived report changed")
	}
}

func TestDeletePreviouslyCheckedReceiptAfterUnchecking(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	r := must(a.Import(ctx, u, "upload", "wrong.png", receiptImage(90), "", nil))
	must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: 500}))
	must(a.UncheckReceipt(ctx, u, r.ID))
	if err := a.DeleteReceipt(ctx, u, r.ID); err != nil {
		t.Fatalf("delete after unchecking: %v", err)
	}
}

func TestDeleteReceiptRequiresOwnership(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, nil)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(91), "", nil))
	other := &User{ID: u.ID + 1, EmployeeID: u.EmployeeID + 1}
	if err := a.DeleteReceipt(ctx, other, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete someone else's receipt: %v", err)
	}
	if _, err := a.GetReceipt(ctx, u, r.ID); err != nil {
		t.Fatalf("owner's receipt missing: %v", err)
	}
}

func TestReceiptNumbersContinueAfterUpgradingAndDeleting(t *testing.T) {
	ctx := context.Background()
	a, _ := newTestApp(t, nil)
	// Build a genuine version-3 database. Later additive migrations can then
	// run normally, rather than undoing tables from the current schema.
	legacyPath := filepath.Join(a.Cfg.DataDir, "legacy.db")
	legacy := must(sql.Open("sqlite", "file:"+legacyPath+"?_pragma=foreign_keys(1)"))
	legacy.SetMaxOpenConns(1)
	t.Cleanup(func() { legacy.Close() })
	provider := must(goose.NewProvider(goose.DialectSQLite3, legacy, os.DirFS("../db/migrations")))
	if _, err := provider.UpTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	a.DB = legacy
	u := must(a.Setup(ctx, "tok", "tok", "Owner", "Max Muster", "a-long-password"))
	key, sum, size := must3(a.Files.Put("files", ".png", bytes.NewReader(receiptImage(92)), 1<<20))
	// A historical checked receipt is fixture data, not created by today's
	// numbering implementation. Its original and number survived the upgrade.
	if _, err := legacy.ExecContext(ctx, `INSERT INTO receipts(id, employee_id, status, source, number, number_year, number_seq, sha256, filename, mime, size, storage_key, preview_key, date, total, created_at)
 VALUES (1, ?, 'checked', 'upload', 'B-2026-0007', 2026, 7, ?, 'receipt.png', 'image/png', ?, ?, ?, '2026-10-05', 500, ?)`, u.EmployeeID, sum, size, key, key, a.stamp()); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	a.DB = must(db.Open(ctx, legacyPath))
	t.Cleanup(func() { a.DB.Close() })
	if err := a.DeleteReceipt(ctx, u, 1); err != nil {
		t.Fatal(err)
	}
	next := must(a.Import(ctx, u, "upload", "next.png", receiptImage(93), "", nil))
	next = must(a.CheckReceipt(ctx, u, next.ID, ReceiptInput{Date: "2026-10-06", Total: 600}))
	if next.Number != "B-2026-0008" {
		t.Fatalf("reused pre-upgrade receipt number: %s", next.Number)
	}
}

func must3[A, B, C any](a A, b B, c C, err error) (A, B, C) {
	if err != nil {
		panic(err)
	}
	return a, b, c
}
