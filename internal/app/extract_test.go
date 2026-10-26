package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestImportedReceiptStartsQueued(t *testing.T) {
	a, u := newTestApp(t, fakeReader{Extraction{Merchant: "Shop"}})
	r := must(a.Import(context.Background(), u, "upload", "receipt.png", receiptImage(140), "", nil))
	if r.Extraction != "queued" || r.Status != "pending" {
		t.Fatalf("new receipt states: %+v", r)
	}
}

type readingResult struct {
	ex  Extraction
	err error
}
type readingCall struct{ result chan readingResult }
type controlledReader struct{ calls chan readingCall }

func (r *controlledReader) Extract(ctx context.Context, _ string, _ []byte) (Extraction, error) {
	call := readingCall{result: make(chan readingResult, 1)}
	select {
	case r.calls <- call:
	case <-ctx.Done():
		return Extraction{}, ctx.Err()
	}
	select {
	case result := <-call.result:
		return result.ex, result.err
	case <-ctx.Done():
		return Extraction{}, ctx.Err()
	}
}

func runReader(t *testing.T, a *App) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.RunExtraction(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("receipt reader did not stop")
		}
	})
	return cancel
}

func nextReading(t *testing.T, reader *controlledReader) readingCall {
	t.Helper()
	select {
	case call := <-reader.calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("receipt reading did not start")
		return readingCall{}
	}
}

func waitExtraction(t *testing.T, a *App, u *User, id int64, state string) *Receipt {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		r := must(a.GetReceipt(context.Background(), u, id))
		if r.Extraction == state {
			return r
		}
		select {
		case <-deadline.C:
			t.Fatalf("receipt %d extraction = %s, want %s", id, r.Extraction, state)
			return nil
		case <-tick.C:
		}
	}
}

func TestReadingStatusAndSerialQueue(t *testing.T) {
	ctx := context.Background()
	reader := &controlledReader{calls: make(chan readingCall, 4)}
	a, u := newTestApp(t, reader)
	first := must(a.Import(ctx, u, "upload", "first.png", receiptImage(141), "", nil))
	second := must(a.Import(ctx, u, "upload", "second.png", receiptImage(142), "", nil))
	runReader(t, a)
	runReader(t, a) // A duplicate start must not create another worker.
	call := nextReading(t, reader)
	first = must(a.GetReceipt(ctx, u, first.ID))
	second = must(a.GetReceipt(ctx, u, second.ID))
	if first.Extraction != "processing" || second.Extraction != "queued" {
		t.Fatalf("first=%s second=%s", first.Extraction, second.Extraction)
	}
	select {
	case <-reader.calls:
		t.Fatal("reader processed two receipts concurrently")
	default:
	}
	must(a.UpdateReceipt(ctx, u, first.ID, ReceiptInput{Merchant: "Manually entered"}))
	call.result <- readingResult{ex: Extraction{Date: "2026-10-05", Total: 700, Merchant: "AI merchant"}}
	first = waitExtraction(t, a, u, first.ID, "done")
	if first.Merchant != "Manually entered" || first.Total != 700 || first.Status != "pending" {
		t.Fatalf("reading overwrote input or checked receipt: %+v", first)
	}
	nextReading(t, reader).result <- readingResult{ex: Extraction{Merchant: "Second"}}
	waitExtraction(t, a, u, second.ID, "done")
}

type countingReader struct{ calls atomic.Int32 }

func (r *countingReader) Extract(context.Context, string, []byte) (Extraction, error) {
	r.calls.Add(1)
	return Extraction{Merchant: "Read"}, nil
}

func TestDurableQueueProcessesMoreThan256Receipts(t *testing.T) {
	ctx := context.Background()
	reader := &countingReader{}
	a, u := newTestApp(t, reader)
	first := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(143), "", nil))
	// A batch fixture shares a validated original to avoid 300 redundant Typst
	// renders. Only public receipt state and reader calls are asserted below.
	if _, err := a.DB.ExecContext(ctx, `WITH RECURSIVE copies(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM copies WHERE n<300)
 INSERT INTO receipts(employee_id, source, sha256, filename, mime, size, pages, storage_key, preview_key, extraction, created_at)
 SELECT r.employee_id, r.source, r.sha256, r.filename, r.mime, r.size, r.pages, r.storage_key, r.preview_key, 'pending', r.created_at
 FROM receipts r, copies WHERE r.id = ?`, first.ID); err != nil {
		t.Fatal(err)
	}
	runReader(t, a)
	waitExtraction(t, a, u, first.ID+300, "done")
	list := must(a.ListReceipts(ctx, u, "", ""))
	if len(list) != 301 || reader.calls.Load() != 301 {
		t.Fatalf("receipts=%d readings=%d", len(list), reader.calls.Load())
	}
	for _, r := range list {
		if r.Extraction != "done" {
			t.Fatalf("stranded receipt %d: %s", r.ID, r.Extraction)
		}
	}
}

func TestFailedReadingRetriesOnceAndPreservesUserInput(t *testing.T) {
	ctx := context.Background()
	reader := &controlledReader{calls: make(chan readingCall, 4)}
	a, u := newTestApp(t, reader)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(144), "", nil))
	runReader(t, a)
	nextReading(t, reader).result <- readingResult{err: errors.New("provider unavailable")}
	r = waitExtraction(t, a, u, r.ID, "failed")
	if r.Extracted.Error != "provider unavailable" {
		t.Fatalf("failure: %+v", r.Extracted)
	}
	must(a.UpdateReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-06", Total: 600, Merchant: "Manual"}))
	r = must(a.RetryExtraction(ctx, u, r.ID))
	call := nextReading(t, reader)
	for range 3 {
		must(a.RetryExtraction(ctx, u, r.ID))
	}
	call.result <- readingResult{ex: Extraction{Date: "2026-10-05", Total: 900, Merchant: "AI"}}
	r = waitExtraction(t, a, u, r.ID, "done")
	if r.Date != "2026-10-06" || r.Total != 600 || r.Merchant != "Manual" || r.Extracted.Error != "" {
		t.Fatalf("retry: %+v", r)
	}
	if _, err := a.RetryExtraction(ctx, u, r.ID); err == nil {
		t.Fatal("retried a successful reading")
	}
	select {
	case <-reader.calls:
		t.Fatal("duplicate retry reading")
	default:
	}
	logs := must(a.ImportLog(ctx, u, 10))
	var failed, succeeded int
	for _, e := range logs {
		if e.Outcome == "read_failed" {
			failed++
		}
		if e.Outcome == "read" {
			succeeded++
		}
	}
	if failed != 1 || succeeded != 1 {
		t.Fatalf("failure logs=%d success logs=%d", failed, succeeded)
	}
}

func TestMissingOriginalFailsInsteadOfRemainingQueued(t *testing.T) {
	ctx := context.Background()
	reader := &countingReader{}
	a, u := newTestApp(t, reader)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(145), "", nil))
	// Simulate an unavailable stored original at the filesystem boundary.
	entries, err := filepath.Glob(filepath.Join(a.Cfg.DataDir, "files", "*", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range entries {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	runReader(t, a)
	r = waitExtraction(t, a, u, r.ID, "failed")
	if reader.calls.Load() != 0 || r.Extracted.Error == "" {
		t.Fatalf("unreadable receipt: %+v, calls=%d", r, reader.calls.Load())
	}
}

func TestCancellationAndRestartRecoverUnfinishedReading(t *testing.T) {
	ctx := context.Background()
	reader := &controlledReader{calls: make(chan readingCall, 4)}
	a, u := newTestApp(t, reader)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(146), "", nil))
	cancel := runReader(t, a)
	nextReading(t, reader)
	cancel()
	waitExtraction(t, a, u, r.ID, "queued")
	// A fresh App has no wake signals or in-memory job IDs from the first run.
	restarted := must(New(a.DB, a.Cfg, a.Log, reader))
	runReader(t, restarted)
	nextReading(t, reader).result <- readingResult{ex: Extraction{Merchant: "Recovered"}}
	r = waitExtraction(t, restarted, u, r.ID, "done")
	if r.Merchant != "Recovered" {
		t.Fatalf("restart: %+v", r)
	}
}

func TestDeletedReceiptCannotOverwriteReusedIDOrCreateReadingLog(t *testing.T) {
	ctx := context.Background()
	reader := &controlledReader{calls: make(chan readingCall, 4)}
	a, u := newTestApp(t, reader)
	old := must(a.Import(ctx, u, "upload", "old.png", receiptImage(147), "", nil))
	runReader(t, a)
	call := nextReading(t, reader)
	if err := a.DeleteReceipt(ctx, u, old.ID); err != nil {
		t.Fatal(err)
	}
	replacement := must(a.Import(ctx, u, "upload", "new.png", receiptImage(148), "", nil))
	if replacement.ID != old.ID {
		t.Fatal("fixture did not reuse the deleted receipt ID")
	}
	call.result <- readingResult{ex: Extraction{Merchant: "Wrong receipt", Total: 900}}
	next := nextReading(t, reader)
	replacement = must(a.GetReceipt(ctx, u, replacement.ID))
	if replacement.Merchant != "" || replacement.Total != 0 || replacement.Extraction != "processing" {
		t.Fatalf("old reading corrupted replacement: %+v", replacement)
	}
	next.result <- readingResult{ex: Extraction{Merchant: "Correct receipt"}}
	waitExtraction(t, a, u, replacement.ID, "done")
	for _, e := range must(a.ImportLog(ctx, u, 10)) {
		if e.Source == "reader" && e.Filename == "old.png" {
			t.Fatal("logged completion for deleted receipt")
		}
	}
}

func TestStartupRecoversClaimLeftByTerminatedProcess(t *testing.T) {
	ctx := context.Background()
	reader := &controlledReader{calls: make(chan readingCall, 2)}
	a, u := newTestApp(t, reader)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(149), "", nil))
	// Simulate a process terminated after durably claiming its receipt.
	if _, err := a.DB.ExecContext(ctx, `UPDATE receipts SET extracting = 1 WHERE id = ?`, r.ID); err != nil {
		t.Fatal(err)
	}
	restarted := must(New(a.DB, a.Cfg, a.Log, reader))
	runReader(t, restarted)
	nextReading(t, reader).result <- readingResult{ex: Extraction{Merchant: "Recovered after termination"}}
	r = waitExtraction(t, restarted, u, r.ID, "done")
	if r.Merchant != "Recovered after termination" {
		t.Fatalf("restart: %+v", r)
	}
}

func TestCheckingDuringReadingKeepsConfirmedValues(t *testing.T) {
	ctx := context.Background()
	reader := &controlledReader{calls: make(chan readingCall, 2)}
	a, u := newTestApp(t, reader)
	r := must(a.Import(ctx, u, "upload", "receipt.png", receiptImage(150), "", nil))
	runReader(t, a)
	call := nextReading(t, reader)
	must(a.CheckReceipt(ctx, u, r.ID, ReceiptInput{Date: "2026-10-06", Total: 600, Merchant: "Confirmed"}))
	call.result <- readingResult{ex: Extraction{Date: "2026-10-05", Total: 900, Merchant: "AI"}}
	r = waitExtraction(t, a, u, r.ID, "done")
	if r.Status != "checked" || r.Merchant != "Confirmed" || r.Date != "2026-10-06" || r.Total != 600 {
		t.Fatalf("checked values overwritten: %+v", r)
	}
	if _, err := a.RetryExtraction(ctx, u, r.ID); err == nil {
		t.Fatal("retried a checked receipt")
	}
}
