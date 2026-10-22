package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/db"
	"github.com/blackdark/vc-beleg-2/internal/export"
)

type testServer struct {
	*httptest.Server
	app *app.App
}

// newServer starts a test server; options adjust the default configuration.
func newServer(t *testing.T, options ...func(*Config)) *testServer {
	t.Helper()
	typst := os.Getenv("MAHLZEIT_TYPST")
	if typst == "" {
		p, err := exec.LookPath("typst")
		if err != nil {
			t.Skip("typst not installed; set MAHLZEIT_TYPST")
		}
		typst = p
	}
	dir := t.TempDir()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fonts := filepath.Join(dir, "fonts")
	if err := export.ExtractFonts(fonts); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(conn, app.Config{DataDir: filepath.Join(dir, "data"), Typst: typst, FontDir: fonts, MaxUploadBytes: 25 << 20, MaxPixels: 40e6, MaxPDFPages: 50}, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Start the listener first so the configuration knows the real base URL
	// (OIDC redirects back to it).
	ts := httptest.NewUnstartedServer(nil)
	cfg := Config{BaseURL: "http://" + ts.Listener.Addr().String(), SetupToken: "setup-token", PasswordEnabled: true, Version: "test"}
	for _, o := range options {
		o(&cfg)
	}
	h, err := New(a, cfg, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = h
	ts.Start()
	t.Cleanup(ts.Close)
	return &testServer{Server: ts, app: a}
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (s *testServer) client(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: s.URL, http: &http.Client{Jar: jar}}
}

// do sends a JSON request and decodes the response into out (if not nil).
func (c *client) do(method, path string, in, out any) int {
	c.t.Helper()
	var body io.Reader
	if b, ok := in.([]byte); ok {
		body = bytes.NewReader(b)
	} else if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, body)
	if _, ok := in.([]byte); ok {
		req.Header.Set("Content-Type", "application/octet-stream")
	} else if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode
}

