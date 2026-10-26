package mailimport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// CheckConnection verifies TLS, authenticates and opens the configured folder
// read-only. It does not search, download, mark, move or delete any messages.
func CheckConnection(ctx context.Context, cfg Config) error {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 15 * time.Second},
		Config:    &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"imap"}},
	}
	return checkConnection(ctx, cfg, dialer.DialContext)
}

// Only the transport boundary is injected: tests still exercise TLS and IMAP.
func checkConnection(ctx context.Context, cfg Config, dial func(context.Context, string, string) (net.Conn, error)) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := dial(ctx, "tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	c := imapclient.New(conn, nil)
	defer c.Close()
	if err := c.WaitGreeting(); err != nil {
		return fmt.Errorf("greeting: %w", err)
	}
	if err := c.Login(cfg.Username, cfg.Password).Wait(); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if _, err := c.Select(cfg.Folder, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return fmt.Errorf("examine: %w", err)
	}
	return nil
}
