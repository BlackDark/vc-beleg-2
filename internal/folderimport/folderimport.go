// Package folderimport polls a WebDAV folder (Nextcloud, ownCloud, a NAS)
// and imports the receipt files placed there. Each processed file is moved
// to a subfolder (or deleted), so it is handled once.
package folderimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// Config configures the watched folder.
type Config struct {
	URL             string // WebDAV URL of the folder
	Username        string
	Password        string
	ProcessedFolder string // subfolder of URL for processed files
	Delete          bool   // delete processed files instead of moving them
	Interval        time.Duration
	MaxBytes        int64
}

// settle is how old a file must be before it is read, so a file still
// being uploaded is left for the next run.
const settle = time.Minute

// receiptExts are the file types imported; anything else stays untouched.
var receiptExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".pdf": true}

// target is the part of the app the importer needs.
type target interface {
	Account(ctx context.Context) (*app.User, error)
	FileImported(ctx context.Context, u *app.User, data []byte) (bool, error)
	Import(ctx context.Context, u *app.User, source, filename string, data []byte, note string, mail *app.MailMeta) (*app.Receipt, error)
}

// folder is the part of the WebDAV client the importer needs.
type folder interface {
	List(ctx context.Context) ([]Entry, error)
	Open(ctx context.Context, href string) (io.ReadCloser, error)
	Mkdir(ctx context.Context, name string) error
	Move(ctx context.Context, href, dest string) error
	Delete(ctx context.Context, href string) error
}

// Tracker remembers files whose server reports no modification date, so
// such a file counts as settled only once a later poll sees it unchanged.
type Tracker struct {
	seen map[string]sighting
}

type sighting struct {
	size int64
	at   time.Time
}

// NewTracker creates an empty tracker.
func NewTracker() *Tracker { return &Tracker{seen: map[string]sighting{}} }

// settled reports whether the file is no longer being written.
func (t *Tracker) settled(fi Entry, now time.Time) bool {
	if !fi.ModTime.IsZero() {
		return now.Sub(fi.ModTime) >= settle
	}
	s, ok := t.seen[fi.Href]
	if !ok || s.size != fi.Size {
		t.seen[fi.Href] = sighting{size: fi.Size, at: now}
		return false
	}
	return now.Sub(s.at) >= settle
}

// forget drops a processed file.
func (t *Tracker) forget(href string) { delete(t.seen, href) }

// Result is one poll's outcome.
type Result struct {
	Imported int
	Rejected []string // files moved away without a receipt (invalid, too large)
}

// Run polls until ctx ends.
func Run(ctx context.Context, cfg Config, a *app.App, log *slog.Logger) {
	a.UpdateFolderStatus(func(s *app.FolderStatus) {
		s.Enabled = true
		s.Folder = Redact(cfg.URL)
	})
	client, err := NewDav(cfg.URL, cfg.Username, cfg.Password)
	if err != nil {
		a.UpdateFolderStatus(func(s *app.FolderStatus) { s.LastError = err.Error() })
		log.Error("folder import disabled", "err", err)
		return
	}
	tracker := NewTracker()
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		res, err := Poll(ctx, cfg, client, a, tracker, time.Now(), log)
		a.UpdateFolderStatus(func(s *app.FolderStatus) {
			s.LastRun = time.Now().UTC().Format(time.RFC3339)
			s.Imported += res.Imported
			s.LastError = ""
			if err != nil {
				s.LastError = err.Error()
			} else if len(res.Rejected) > 0 {
				s.LastError = "not imported: " + strings.Join(res.Rejected, ", ")
			}
		})
		if err != nil && ctx.Err() == nil {
			log.Warn("folder import failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Poll imports the settled receipt files of the folder once.
func Poll(ctx context.Context, cfg Config, f folder, t target, tr *Tracker, now time.Time, log *slog.Logger) (Result, error) {
	var res Result
	u, err := t.Account(ctx)
	if errors.Is(err, app.ErrNotFound) {
		return res, nil // no account yet
	}
	if err != nil {
		return res, err
	}
	files, err := candidates(ctx, f, tr, now)
	if err != nil || len(files) == 0 {
		return res, err
	}
	if !cfg.Delete {
		// Fails when the folder exists, which is fine; a real problem shows
		// up when moving.
		_ = f.Mkdir(ctx, cfg.ProcessedFolder)
	}
	for _, fi := range files {
		if ctx.Err() != nil {
			return res, nil
		}
		name := fi.Name
		ok, err := importFile(ctx, cfg, f, t, u, fi)
		if err != nil {
			return res, err
		}
		if ok {
			res.Imported++
		} else {
			res.Rejected = append(res.Rejected, name)
			log.Warn("folder import: file rejected", "file", name)
		}
		if err := finish(ctx, cfg, f, fi, now); err != nil {
			return res, err
		}
		tr.forget(fi.Href)
	}
	return res, nil
}

// candidates lists the receipt files that are no longer being written.
func candidates(ctx context.Context, f folder, tr *Tracker, now time.Time) ([]Entry, error) {
	list, err := f.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list folder: %w", err)
	}
	var out []Entry
	for _, fi := range list {
		ext := strings.ToLower(path.Ext(fi.Name))
		if fi.IsDir || !receiptExts[ext] || !tr.settled(fi, now) {
			continue
		}
		out = append(out, fi)
	}
	return out, nil
}

// importFile reads one file and imports it unless the same content came
// from the folder before (a move that failed last time). It reports whether
// the file is accounted for; false means it was rejected.
func importFile(ctx context.Context, cfg Config, f folder, t target, u *app.User, fi Entry) (bool, error) {
	if fi.Size > cfg.MaxBytes {
		return false, nil
	}
	r, err := f.Open(ctx, fi.Href)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", fi.Name, err)
	}
	data, err := io.ReadAll(io.LimitReader(r, cfg.MaxBytes+1))
	r.Close()
	if err != nil {
		return false, fmt.Errorf("read %s: %w", fi.Name, err)
	}
	if int64(len(data)) > cfg.MaxBytes {
		return false, nil // the server did not report the size
	}
	if done, err := t.FileImported(ctx, u, data); err != nil || done {
		return done, err
	}
	_, err = t.Import(ctx, u, "folder", fi.Name, data, "", nil)
	var invalid *app.Error
	if errors.As(err, &invalid) {
		return false, nil // not a receipt the app accepts
	}
	return err == nil, err
}

// finish deletes the file or moves it to the processed folder under a
// time-stamped name, so files with the same name do not collide.
func finish(ctx context.Context, cfg Config, f folder, fi Entry, now time.Time) error {
	if cfg.Delete {
		if err := f.Delete(ctx, fi.Href); err != nil {
			return fmt.Errorf("delete %s: %w", fi.Name, err)
		}
		return nil
	}
	dest := path.Join(cfg.ProcessedFolder, now.Format("20060102-150405")+"-"+fi.Name)
	if err := f.Move(ctx, fi.Href, dest); err != nil {
		return fmt.Errorf("move %s: %w", fi.Name, err)
	}
	return nil
}

// Redact returns the URL without credentials for display and logs.
func Redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	return u.String()
}
