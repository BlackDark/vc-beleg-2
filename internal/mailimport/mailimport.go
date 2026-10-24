// Package mailimport polls a dedicated IMAP mailbox and imports receipt
// attachments from allowed senders. Each processed mail is moved to a folder
// (or deleted), so it is handled once.
package mailimport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	_ "github.com/emersion/go-message/charset" // decode non-UTF-8 headers
	"github.com/emersion/go-message/mail"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// Config configures the mailbox.
type Config struct {
	Host            string
	Port            int
	Username        string
	Password        string
	Folder          string
	ProcessedFolder string
	Delete          bool // delete processed mails instead of moving them
	AllowedSenders  []string
	Interval        time.Duration
}

// Attachment is a receipt file found in a mail.
type Attachment struct {
	Filename string
	Data     []byte
}

// Mail is a parsed message.
type Mail struct {
	MessageID   string
	From        string
	Subject     string
	Attachments []Attachment
}

// Run polls until ctx ends.
func Run(ctx context.Context, cfg Config, a *app.App, log *slog.Logger) {
	a.UpdateMailStatus(func(s *app.MailStatus) {
		s.Enabled = true
		s.Mailbox = fmt.Sprintf("%s@%s/%s", cfg.Username, cfg.Host, cfg.Folder)
	})
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		n, err := poll(ctx, cfg, a, log)
		a.UpdateMailStatus(func(s *app.MailStatus) {
			s.LastRun = time.Now().UTC().Format(time.RFC3339)
			s.Imported += n
			s.LastError = ""
			if err != nil {
				s.LastError = err.Error()
			}
		})
		if err != nil && ctx.Err() == nil {
			log.Warn("mail import failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// poll processes the unread mails once and returns the receipts imported.
func poll(ctx context.Context, cfg Config, a *app.App, log *slog.Logger) (int, error) {
	u, err := a.Account(ctx)
	if errors.Is(err, app.ErrNotFound) {
		return 0, nil // no account yet
	}
	if err != nil {
		return 0, err
	}
	c, err := imapclient.DialTLS(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), nil)
	if err != nil {
		return 0, fmt.Errorf("connect: %w", err)
	}
	defer c.Close()
	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return 0, fmt.Errorf("login: %w", err)
	}
	defer c.Logout()
	if _, err := c.Select(cfg.Folder, nil).Wait(); err != nil {
		return 0, fmt.Errorf("select %s: %w", cfg.Folder, err)
	}
	found, err := c.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		return 0, fmt.Errorf("search: %w", err)
	}
	if !cfg.Delete && len(found.AllUIDs()) > 0 {
		// Fails when the folder exists, which is fine.
		_ = c.Create(cfg.ProcessedFolder, nil).Wait()
	}
	imported := 0
	for _, uid := range found.AllUIDs() {
		if ctx.Err() != nil {
			return imported, nil
		}
		set := imap.UIDSetNum(uid)
		section := &imap.FetchItemBodySection{Peek: true}
		msgs, err := c.Fetch(set, &imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{section}}).Collect()
		if err != nil || len(msgs) == 0 {
			return imported, fmt.Errorf("fetch: %w", err)
		}
		m, err := Parse(msgs[0].FindBodySection(section))
		if err != nil {
			log.Warn("mail import: unreadable mail", "uid", uid, "err", err)
		} else {
			imported += importMail(ctx, a, u, cfg, m, log)
		}
		if err := finish(c, set, cfg); err != nil {
			return imported, err
		}
	}
	if n := len(found.AllUIDs()); n > 0 {
		log.Info("mail import run", "mails", n, "imported", imported)
	}
	return imported, nil
}

// importMail imports the attachments of an allowed sender and returns how
// many receipts were created. Failures of single files are logged; the mail
// still counts as processed.
func importMail(ctx context.Context, a *app.App, u *app.User, cfg Config, m *Mail, log *slog.Logger) int {
	log.Info("mail import: mail", "attachments", len(m.Attachments))
	if !Allowed(cfg.AllowedSenders, m.From) {
		log.Info("mail import: sender not allowed", "from", m.From)
		return 0
	}
	if done, err := a.MailImported(ctx, u, m.MessageID); err != nil || done {
		return 0
	}
	n := 0
	for _, att := range m.Attachments {
		if _, err := a.Import(ctx, u, "mail", att.Filename, att.Data, m.Subject, &app.MailMeta{MessageID: m.MessageID, From: m.From}); err != nil {
			log.Warn("mail import: attachment rejected", "file", att.Filename, "err", err)
			continue
		}
		n++
	}
	return n
}

// finish marks the mail read and moves or deletes it.
func finish(c *imapclient.Client, set imap.UIDSet, cfg Config) error {
	flags := []imap.Flag{imap.FlagSeen}
	if cfg.Delete {
		flags = append(flags, imap.FlagDeleted)
	}
	if err := c.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: flags, Silent: true}, nil).Close(); err != nil {
		return fmt.Errorf("store flags: %w", err)
	}
	if cfg.Delete {
		return c.UIDExpunge(set).Close()
	}
	if _, err := c.Move(set, cfg.ProcessedFolder).Wait(); err != nil {
		return fmt.Errorf("move to %s: %w", cfg.ProcessedFolder, err)
	}
	return nil
}

// Allowed reports whether from is on the allowlist (case-insensitive).
func Allowed(allowed []string, from string) bool {
	return from != "" && slices.ContainsFunc(allowed, func(s string) bool { return strings.EqualFold(strings.TrimSpace(s), from) })
}

var receiptTypes = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "application/pdf": ".pdf"}

// Parse reads sender, subject and receipt attachments (JPEG, PNG, PDF) of
// a raw message. Inline images count too: phones often send photos inline.
func Parse(raw []byte) (*Mail, error) {
	r, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	m := &Mail{}
	m.MessageID, _ = r.Header.MessageID()
	m.Subject, _ = r.Header.Subject()
	if from, err := r.Header.AddressList("From"); err == nil && len(from) > 0 {
		m.From = strings.ToLower(from[0].Address)
	}
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, err
		}
		var name, ctype string
		switch h := p.Header.(type) {
		case *mail.AttachmentHeader:
			name, _ = h.Filename()
			ctype, _, _ = h.ContentType()
		case *mail.InlineHeader:
			ctype, _, _ = h.ContentType()
			if _, params, err := h.ContentDisposition(); err == nil {
				name = params["filename"]
			}
		}
		ctype = strings.ToLower(ctype)
		if ctype == "application/octet-stream" {
			ctype = mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		}
		ext, ok := receiptTypes[ctype]
		if !ok {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(p.Body, 30<<20))
		if err != nil {
			return m, err
		}
		if name == "" {
			name = fmt.Sprintf("mail-%d%s", len(m.Attachments)+1, ext)
		}
		m.Attachments = append(m.Attachments, Attachment{Filename: name, Data: data})
	}
	return m, nil
}
