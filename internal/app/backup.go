package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Backup writes a consistent copy of the database (VACUUM INTO) and the
// stored originals and exports to dest. Files are always written before the
// rows referencing them, so copying the database first and the files second
// yields a complete set even while the server runs.
func Backup(ctx context.Context, db *sql.DB, dataDir, dest string) error {
	if _, err := os.Stat(filepath.Join(dest, "mahlzeit.db")); err == nil {
		return errors.New(dest + " already contains a backup")
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, filepath.Join(dest, "mahlzeit.db")); err != nil {
		return err
	}
	for _, area := range []string{"files", "exports"} {
		if err := copyTree(filepath.Join(dataDir, area), filepath.Join(dest, area)); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
