package folderimport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav"
	xwebdav "golang.org/x/net/webdav"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// fakeApp records imports; files whose content starts with "bad" are
// rejected like an unreadable image.
type fakeApp struct {
	imported []string
	seen     map[string]bool
}

func (f *fakeApp) Account(context.Context) (*app.User, error) { return &app.User{ID: 1}, nil }

func (f *fakeApp) FileImported(_ context.Context, _ *app.User, data []byte) (bool, error) {
	return f.seen[string(data)], nil
}

func (f *fakeApp) Import(_ context.Context, _ *app.User, source, filename string, data []byte, _ string, _ *app.MailMeta) (*app.Receipt, error) {
	if source != "folder" {
		panic("source " + source)
	}
	if strings.HasPrefix(string(data), "bad") {
		return nil, app.Invalid("upload.unreadable", "unreadable")
	}
	f.imported = append(f.imported, filename)
	return &app.Receipt{}, nil
}

// newDAV serves an in-memory WebDAV tree under /dav/ with basic auth and
// returns a client for the folder /dav/Belege and its URL.
func newDAV(t *testing.T) (*webdav.Client, xwebdav.FileSystem, string) {
	t.Helper()
	fs := xwebdav.NewMemFS()
	if err := fs.Mkdir(context.Background(), "/Belege", 0o755); err != nil {
		t.Fatal(err)
	}
	h := &xwebdav.Handler{Prefix: "/dav", FileSystem: fs, LockSystem: xwebdav.NewMemLS()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "max" || p != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/dav/Belege"
	c, err := webdav.NewClient(webdav.HTTPClientWithBasicAuth(nil, "max", "secret"), url)
	if err != nil {
		t.Fatal(err)
	}
	return c, fs, url
}

func put(t *testing.T, fs xwebdav.FileSystem, name, content string) {
	t.Helper()
	f, err := fs.OpenFile(context.Background(), name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, content); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func names(t *testing.T, fs xwebdav.FileSystem, dir string) []string {
	t.Helper()
	f, err := fs.OpenFile(context.Background(), dir, os.O_RDONLY, 0)
	if err != nil {
		return nil
	}
	defer f.Close()
	infos, _ := f.Readdir(-1)
	var out []string
	for _, fi := range infos {
		out = append(out, fi.Name())
	}
	slices.Sort(out)
	return out
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestPollMovesProcessedFiles(t *testing.T) {
	c, fs, _ := newDAV(t)
	put(t, fs, "/Belege/bon 1.JPG", "jpeg")
	put(t, fs, "/Belege/scan.pdf", "pdf")
	put(t, fs, "/Belege/kaputt.png", "bad png")
	put(t, fs, "/Belege/notes.txt", "keep me")
	cfg := Config{ProcessedFolder: "Verarbeitet", MaxBytes: 1 << 20}
	now := time.Now().Add(2 * time.Minute) // files are settled
	a := &fakeApp{}

	res, err := Poll(context.Background(), cfg, c, a, now, quiet)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(a.imported)
	if res.Imported != 2 || !slices.Equal(a.imported, []string{"bon 1.JPG", "scan.pdf"}) || !slices.Equal(res.Rejected, []string{"kaputt.png"}) {
		t.Fatalf("result = %+v, imported %v", res, a.imported)
	}
	stamp := now.Format("20060102-150405") + "-"
	if got := names(t, fs, "/Belege"); !slices.Equal(got, []string{"Verarbeitet", "notes.txt"}) {
		t.Fatalf("folder = %v", got)
	}
	if got := names(t, fs, "/Belege/Verarbeitet"); !slices.Equal(got, []string{stamp + "bon 1.JPG", stamp + "kaputt.png", stamp + "scan.pdf"}) {
		t.Fatalf("processed = %v", got)
	}
}

func TestPollSkipsUnsettledAndDuplicatesAndDeletes(t *testing.T) {
	c, fs, _ := newDAV(t)
	put(t, fs, "/Belege/neu.jpg", "fresh")
	put(t, fs, "/Belege/alt.jpg", "again")
	put(t, fs, "/Belege/gross.pdf", strings.Repeat("x", 200))
	cfg := Config{Delete: true, MaxBytes: 100}
	a := &fakeApp{seen: map[string]bool{"again": true}}

	// Just written: nothing happens yet.
	if res, err := Poll(context.Background(), cfg, c, a, time.Now(), quiet); err != nil || res.Imported != 0 || len(names(t, fs, "/Belege")) != 3 {
		t.Fatalf("unsettled: %+v %v", res, err)
	}
	res, err := Poll(context.Background(), cfg, c, a, time.Now().Add(2*time.Minute), quiet)
	if err != nil {
		t.Fatal(err)
	}
	// The duplicate counts as done; the oversized file is rejected; all go.
	if res.Imported != 2 || !slices.Equal(a.imported, []string{"neu.jpg"}) || !slices.Equal(res.Rejected, []string{"gross.pdf"}) {
		t.Fatalf("result = %+v, imported %v", res, a.imported)
	}
	if got := names(t, fs, "/Belege"); len(got) != 0 {
		t.Fatalf("folder = %v", got)
	}
}

func TestPollReportsWrongPassword(t *testing.T) {
	_, fs, url := newDAV(t)
	put(t, fs, "/Belege/a.jpg", "jpeg")
	bad, err := webdav.NewClient(webdav.HTTPClientWithBasicAuth(nil, "max", "wrong"), url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Poll(context.Background(), Config{MaxBytes: 1}, bad, &fakeApp{}, time.Now(), quiet); err == nil || !strings.Contains(err.Error(), "list folder") {
		t.Fatalf("err = %v", err)
	}
}

func TestRedact(t *testing.T) {
	if got := Redact("https://max:secret@cloud.example.com/remote.php/dav/files/max/Belege"); got != "https://cloud.example.com/remote.php/dav/files/max/Belege" {
		t.Fatalf("redact = %q", got)
	}
}
