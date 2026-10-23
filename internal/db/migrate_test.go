package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// A receipt stored under the first schema survives the rebuild that allows
// the folder source.
func TestFolderSourceMigrationKeepsReceipts(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "m.db")
	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, conn, mustSub(migrations, "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users(id, name, created_at) VALUES (1, 'Max', 'x')`,
		`INSERT INTO companies(id, created_at) VALUES (1, 'x')`,
		`INSERT INTO employees(id, company_id, user_id, name, created_at) VALUES (1, 1, 1, 'Max', 'x')`,
		`INSERT INTO receipts(employee_id, source, sha256, filename, mime, size, storage_key, preview_key, merchant, created_at)
			VALUES (1, 'mail', 'abc', 'bon.png', 'image/png', 3, 'k', 'p', 'REWE', 'x')`,
	} {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	conn.Close()

	conn, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var merchant, source string
	if err := conn.QueryRowContext(ctx, `SELECT merchant, source FROM receipts`).Scan(&merchant, &source); err != nil || merchant != "REWE" || source != "mail" {
		t.Fatalf("receipt = %q %q, %v", merchant, source, err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO receipts(employee_id, source, sha256, filename, mime, size, storage_key, preview_key, created_at)
		VALUES (1, 'folder', 'def', 'scan.pdf', 'application/pdf', 3, 'k2', 'p2', 'x')`); err != nil {
		t.Fatalf("folder source rejected: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO receipts(employee_id, source, sha256, filename, mime, size, storage_key, preview_key, created_at)
		VALUES (1, 'fax', 'ghi', 'x.pdf', 'application/pdf', 3, 'k3', 'p3', 'x')`); err == nil {
		t.Fatal("unknown source accepted")
	}
}
