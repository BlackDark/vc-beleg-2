// Package db opens the SQLite database and applies embedded migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // cgo-free driver
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (and creates) the database at path in WAL mode with foreign
// keys enabled, and migrates it to the latest schema.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_txlock=immediate", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection serializes writes, so closing a month sees a stable set
	// of receipts.
	conn.SetMaxOpenConns(1)
	if err := conn.PingContext(ctx); err != nil {
		return nil, err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, mustSub(migrations, "migrations"))
	if err != nil {
		return nil, err
	}
	if _, err := provider.Up(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return conn, nil
}
