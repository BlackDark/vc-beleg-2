package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// Files stores originals and exports under generated keys. Keys never come
// from user input, so paths cannot escape the data directory.
type Files struct {
	root string
}

var keyPattern = regexp.MustCompile(`^(files|exports)/[0-9a-f]{2}/[0-9a-f]{32}(\.[a-z]{2,4})?$`)

// NewFiles prepares the storage directories.
func NewFiles(dataDir string) (*Files, error) {
	f := &Files{root: dataDir}
	for _, d := range []string{"files", "exports"} {
		if err := os.MkdirAll(filepath.Join(dataDir, d), 0o750); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// newKey generates a storage key in area ("files" or "exports").
func newKey(area, ext string) string {
	id := randomHex(16)
	return fmt.Sprintf("%s/%s/%s%s", area, id[:2], id, ext)
}

func (f *Files) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", errors.New("invalid storage key")
	}
	return filepath.Join(f.root, filepath.FromSlash(key)), nil
}

// Put stores r under a new key; it returns the key, SHA-256 and size.
// It fails when more than limit bytes are supplied.
func (f *Files) Put(area, ext string, r io.Reader, limit int64) (key, sum string, size int64, err error) {
	key = newKey(area, ext)
	p, err := f.path(key)
	if err != nil {
		return "", "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return "", "", 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return "", "", 0, err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, limit+1))
	if err == nil && n > limit {
		err = Invalid("upload.too_large", "file exceeds the size limit")
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", "", 0, err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return "", "", 0, err
	}
	return key, hex.EncodeToString(h.Sum(nil)), n, nil
}

// Open opens a stored file.
func (f *Files) Open(key string) (*os.File, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

// Read returns a stored file's bytes.
func (f *Files) Read(key string) ([]byte, error) {
	p, err := f.path(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Remove deletes a stored file (unchecked uploads, failed reports).
func (f *Files) Remove(key string) {
	if p, err := f.path(key); err == nil {
		_ = os.Remove(p)
	}
}

// SHA256 computes a stored file's checksum.
func (f *Files) SHA256(key string) (string, error) {
	r, err := f.Open(key)
	if err != nil {
		return "", err
	}
	defer r.Close()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
