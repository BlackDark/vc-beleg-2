package app

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/db"
	"github.com/blackdark/vc-beleg-2/internal/money"
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
	a, u := newTestApp(t, nil)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(92), "", nil))
	r = must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-05", Total: 500}))
	// Recreate the pre-counter schema as an upgrade fixture. Assertions below
	// observe only the app's public receipt operations and assigned numbers.
	if _, err := a.DB.ExecContext(ctx, `DROP TABLE receipt_sequences; DELETE FROM goose_db_version WHERE version_id = 4`); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.Close(); err != nil {
		t.Fatal(err)
	}
	a.DB = must(db.Open(ctx, filepath.Join(a.Cfg.DataDir, "test.db")))
	t.Cleanup(func() { a.DB.Close() })
	if err := a.DeleteReceipt(ctx, u, r.ID); err != nil {
		t.Fatal(err)
	}
	next := must(a.Import(ctx, u, "upload", "next.png", receiptImage(93), "", nil))
	next = must(a.CheckReceipt(ctx, u, next.ID, ReceiptInput{Date: "2026-10-06", Total: 600}))
	if next.Number != "B-2026-0002" {
		t.Fatalf("reused pre-upgrade receipt number: %s", next.Number)
	}
}
