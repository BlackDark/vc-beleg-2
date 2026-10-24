package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/folderimport"
)

// check runs diagnostics against the live configuration without changing
// any data: "check folder" or "check ai <file>".
func check(args []string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	switch {
	case len(args) == 1 && args[0] == "folder":
		return checkFolder(ctx, cfg)
	case len(args) == 2 && args[0] == "ai":
		return checkAI(ctx, cfg, args[1])
	}
	return errors.New("usage: mahlzeit check folder | mahlzeit check ai <file>")
}

// checkFolder prints what the next import run would do with each file.
func checkFolder(ctx context.Context, cfg *Config) error {
	if cfg.Folder == nil {
		return errors.New("folder import is off (MAHLZEIT_WEBDAV_URL)")
	}
	dav, err := folderimport.NewDav(cfg.Folder.URL, cfg.Folder.Username, cfg.Folder.Password)
	if err != nil {
		return err
	}
	fmt.Println("Folder:", folderimport.Redact(cfg.Folder.URL))
	found, err := folderimport.Inspect(ctx, dav, cfg.Folder.MaxBytes, time.Now())
	if err != nil {
		return err
	}
	if len(found) == 0 {
		fmt.Println("The folder is empty.")
	}
	for _, f := range found {
		size, date := "size unknown", "no date"
		if f.Size >= 0 {
			size = fmt.Sprintf("%d bytes", f.Size)
		}
		if !f.Modified.IsZero() {
			date = f.Modified.Local().Format("2006-01-02 15:04")
		}
		fmt.Printf("- %s (%s, %s)\n  %s\n", f.Name, size, date, f.Verdict)
	}
	fmt.Println("Nothing was imported, moved or deleted.")
	return nil
}

// checkAI reads one receipt file with the configured reader and prints the
// result; nothing is stored.
func checkAI(ctx context.Context, cfg *Config, path string) error {
	reader := newReader(cfg.Extract)
	if reader == nil {
		return errors.New("receipt reading is off (see MAHLZEIT_EXTRACT_PROVIDER in docs/OPERATIONS.md)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	mime, ok := app.DetectType(data)
	if !ok {
		return errors.New("not a JPEG, PNG or PDF file")
	}
	fmt.Println("Reader:", cfg.Extract.Label())
	start := time.Now()
	ex, err := reader.Extract(ctx, mime, data)
	fmt.Println("Took:", time.Since(start).Round(100*time.Millisecond))
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(ex, "", "  ")
	fmt.Println(string(out))
	return nil
}
