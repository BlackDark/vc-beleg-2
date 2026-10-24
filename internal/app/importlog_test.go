package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestImportLog(t *testing.T) {
	ctx := context.Background()
	a, u := newTestApp(t, fakeReader{Extraction{Date: "2026-10-09", Total: 789}})
	a.Cfg.ReaderLabel = "gemini gemini-flash-latest"
	r := must(a.Import(ctx, u, "folder", "bon.png", receiptImage(1), "", nil))
	if _, err := a.Import(ctx, u, "mail", "foto.heic", []byte("\x00\x00\x00\x18ftypheic"), "", nil); err == nil {
		t.Fatal("HEIC accepted")
	}
	a.extractOne(ctx, <-a.extractQueue)

	log := must(a.ImportLog(ctx, u, 10))
	if len(log) != 3 {
		t.Fatalf("log = %+v", log)
	}
	// Newest first: the reading, the rejection, the import.
	read, rejected, imported := log[0], log[1], log[2]
	if imported.Outcome != "imported" || imported.Source != "folder" || imported.ReceiptID != r.ID || imported.Filename != "bon.png" {
		t.Errorf("imported = %+v", imported)
	}
	if rejected.Outcome != "rejected" || rejected.Source != "mail" || rejected.ReceiptID != 0 || !strings.Contains(rejected.Message, "JPEG, PNG and PDF") || rejected.Code != "upload.unsupported" {
		t.Errorf("rejected = %+v", rejected)
	}
	if read.Outcome != "read" || read.ReceiptID != r.ID || !strings.HasPrefix(read.Message, "gemini gemini-flash-latest · ") || !strings.HasSuffix(read.Message, "found: date, total") {
		t.Errorf("read = %+v", read)
	}

	// Entries older than 90 days go with the next write.
	a.now = func() time.Time { return time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC) }
	a.Record(ctx, u, LogEntry{Source: "upload", Filename: "neu.png", Outcome: "imported"})
	if log := must(a.ImportLog(ctx, u, 10)); len(log) != 1 || log[0].Filename != "neu.png" {
		t.Fatalf("after retention: %+v", log)
	}
}
