// Command mahlzeit runs the meal allowance server and its maintenance tasks.
//
//	mahlzeit [serve]      run the server
//	mahlzeit openapi      print the OpenAPI document
//	mahlzeit backup DIR   write a consistent backup to DIR
//	mahlzeit verify       check checksums of receipts and reports
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/api"
	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/db"
	"github.com/blackdark/vc-beleg-2/internal/export"
	"github.com/blackdark/vc-beleg-2/internal/extract"
	"github.com/blackdark/vc-beleg-2/internal/mailimport"
	"github.com/blackdark/vc-beleg-2/web"
	_ "time/tzdata" // report time stamps in Europe/Berlin on a minimal image
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "openapi":
		err = printOpenAPI()
	case "backup":
		err = backup(os.Args[2:])
	case "verify":
		err = verify()
	case "healthcheck":
		err = healthcheck()
	case "version", "--version":
		fmt.Println(version)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func logger(cfg *Config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
}

func serve() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	log := logger(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return err
	}
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "mahlzeit.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	fontDir, err := os.MkdirTemp("", "mahlzeit-fonts-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(fontDir)
	if err := export.ExtractFonts(fontDir); err != nil {
		return err
	}
	var reader app.Extractor
	if cfg.AnthropicKey != "" {
		reader = extract.New(cfg.AnthropicKey, cfg.ExtractModel, cfg.ExtractFallback)
	}
	a, err := app.New(conn, app.Config{DataDir: cfg.DataDir, Typst: cfg.Typst, FontDir: fontDir, MaxUploadBytes: cfg.MaxUploadBytes, MaxPixels: cfg.MaxPixels, MaxPDFPages: cfg.MaxPDFPages}, log, reader)
	if err != nil {
		return err
	}
	go a.RunExtraction(ctx)
	if cfg.Mail != nil {
		go mailimport.Run(ctx, *cfg.Mail, a, log)
	}
	if err := ensureSetupToken(ctx, a, cfg, log); err != nil {
		return err
	}
	handler, err := api.New(a, api.Config{BaseURL: cfg.BaseURL, SecureCookies: cfg.SecureCookies, SetupToken: cfg.SetupToken, PasswordEnabled: cfg.PasswordLogin, OIDC: cfg.OIDC, Version: version}, log, web.Handler())
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("listening", "addr", cfg.Listen, "version", version, "password", cfg.PasswordLogin, "oidc", cfg.OIDC != nil, "extraction", reader != nil, "mail", cfg.Mail != nil)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// ensureSetupToken writes a one-time token file when no account exists,
// password login (which setup needs) is on and the operator did not supply
// MAHLZEIT_SETUP_TOKEN.
func ensureSetupToken(ctx context.Context, a *app.App, cfg *Config, log *slog.Logger) error {
	if !cfg.PasswordLogin || cfg.SetupToken != "" {
		return nil
	}
	needs, err := a.NeedsSetup(ctx)
	if err != nil || !needs {
		return err
	}
	cfg.SetupToken = randomToken()
	p := filepath.Join(cfg.DataDir, "setup-token")
	if err := os.WriteFile(p, []byte(cfg.SetupToken+"\n"), 0o600); err != nil {
		return err
	}
	log.Warn("no account yet: the setup token was written to a file", "path", p)
	return nil
}

func printOpenAPI() error {
	b, err := json.MarshalIndent(api.NewForSpec(version).OpenAPI(), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// backup writes a consistent SQLite copy (VACUUM INTO) and copies the
// referenced files. Run it while the server is running or stopped.
func backup(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: mahlzeit backup DIR")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	dest := args[0]
	if _, err := os.Stat(filepath.Join(dest, "mahlzeit.db")); err == nil {
		return fmt.Errorf("%s already contains a backup", dest)
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "mahlzeit.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := app.Backup(ctx, conn, cfg.DataDir, dest); err != nil {
		return err
	}
	fmt.Println("backup written to", dest)
	return nil
}

// verify checks every stored original and report against its recorded
// SHA-256; run it after a restore.
func verify() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(cfg.DataDir, "mahlzeit.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	files, err := app.NewFiles(cfg.DataDir)
	if err != nil {
		return err
	}
	r, err := (&app.App{DB: conn, Files: files}).Verify(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("checked %d receipt originals and %d reports\n", r.Receipts, r.Reports)
	for _, p := range r.Problems {
		fmt.Println("PROBLEM:", p)
	}
	if len(r.Problems) > 0 {
		return fmt.Errorf("%d problems found", len(r.Problems))
	}
	fmt.Println("all checksums match")
	return nil
}

func healthcheck() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 3 * time.Second}
	addr := cfg.Listen
	if addr != "" && addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
