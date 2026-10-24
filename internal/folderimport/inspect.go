package folderimport

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // decode receipt images
	_ "image/png"
	"io"
	"net/http"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// Finding is what an import run would do with one entry of the folder.
type Finding struct {
	Name     string
	Size     int64 // -1 when the server does not report it
	Modified time.Time
	Verdict  string
}

// Inspect lists the folder and downloads every receipt file to say what an
// import run would do, without importing, moving or deleting anything. The
// full checks (preview rendering, PDF pages) run only on a real import.
func Inspect(ctx context.Context, f folder, maxBytes int64, now time.Time) ([]Finding, error) {
	list, err := f.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list folder: %w", err)
	}
	out := make([]Finding, 0, len(list))
	for _, fi := range list {
		out = append(out, Finding{Name: fi.Name, Size: fi.Size, Modified: fi.ModTime, Verdict: inspectEntry(ctx, f, fi, maxBytes, now)})
	}
	return out, nil
}

func inspectEntry(ctx context.Context, f folder, fi Entry, maxBytes int64, now time.Time) string {
	switch {
	case fi.IsDir:
		return "skipped: folder"
	case !isReceiptFile(fi):
		return "skipped: not .jpg, .jpeg, .png or .pdf (stays untouched)"
	case fi.Size > maxBytes:
		return "would reject: larger than the upload limit"
	}
	r, err := f.Open(ctx, fi.Href)
	if err != nil {
		return "download failed: " + err.Error()
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	r.Close()
	if err != nil {
		return "download failed: " + err.Error()
	}
	if int64(len(data)) > maxBytes {
		return "would reject: larger than the upload limit"
	}
	wait := ""
	if fi.ModTime.IsZero() {
		wait = "; no date from the server, so imported one run after it is first seen"
	} else if now.Sub(fi.ModTime) < settle {
		wait = "; changed less than a minute ago, so left for the next run"
	}
	mime, ok := app.DetectType(data)
	if !ok {
		return fmt.Sprintf("would reject: not a JPEG, PNG or PDF (content looks like %s, %d bytes)%s", http.DetectContentType(data), len(data), wait)
	}
	if mime == "application/pdf" {
		return fmt.Sprintf("would import: PDF, %d bytes%s", len(data), wait)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Sprintf("would reject: %s could not be read (%d bytes)%s", mime, len(data), wait)
	}
	return fmt.Sprintf("would import: %s %dx%d, %d bytes%s", mime, cfg.Width, cfg.Height, len(data), wait)
}
