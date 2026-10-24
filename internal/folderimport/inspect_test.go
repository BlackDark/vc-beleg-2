package folderimport

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestInspectChangesNothing(t *testing.T) {
	c, fs, _ := newDAV(t)
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 30, 40))); err != nil {
		t.Fatal(err)
	}
	put(t, fs, "/Belege/bon.png", img.String())
	put(t, fs, "/Belege/Mittel (PNG-Bild).jpeg", "\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic") // iPhone HEIC
	put(t, fs, "/Belege/login.jpg", "<!DOCTYPE html><html><body>Login</body></html>")
	put(t, fs, "/Belege/notes.txt", "keep")
	before := names(t, fs, "/Belege")

	got, err := Inspect(context.Background(), c, 1<<20, time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	verdict := map[string]string{}
	for _, f := range got {
		verdict[f.Name] = f.Verdict
	}
	for name, want := range map[string]string{
		"bon.png":                "would import: image/png 30x40",
		"Mittel (PNG-Bild).jpeg": "would reject: not a JPEG, PNG or PDF",
		"login.jpg":              "content looks like text/html",
		"notes.txt":              "skipped: not .jpg",
	} {
		if !strings.Contains(verdict[name], want) {
			t.Errorf("%s: %q, want %q", name, verdict[name], want)
		}
	}
	if after := names(t, fs, "/Belege"); !slices.Equal(before, after) {
		t.Fatalf("folder changed: %v -> %v", before, after)
	}
}
