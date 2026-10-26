package mailimport

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// The fixture speaks IMAP over a verified TLS connection, including failures.
func imapDiagnosticServer(t *testing.T, mode string) (Config, func(context.Context, string, string) (net.Conn, error), <-chan []string) {
	t.Helper()
	seed := httptest.NewTLSServer(nil)
	serverTLS := seed.TLS.Clone()
	roots := x509.NewCertPool()
	roots.AddCert(seed.Certificate())
	seed.Close()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	done := make(chan []string, 1)
	go func() {
		var commands []string
		defer func() { done <- commands }()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := fmt.Fprint(conn, "* OK [CAPABILITY IMAP4rev1] test ready\r\n"); err != nil {
			return
		}
		scan := bufio.NewScanner(conn)
		for scan.Scan() {
			fields := strings.Fields(scan.Text())
			if len(fields) < 2 {
				return
			}
			tag, command := fields[0], fields[1]
			commands = append(commands, command)
			switch command {
			case "CAPABILITY":
				fmt.Fprintf(conn, "* CAPABILITY IMAP4rev1\r\n%s OK capability\r\n", tag)
			case "LOGIN":
				if mode == "reject" {
					fmt.Fprintf(conn, "%s NO authentication failed\r\n", tag)
				} else if mode != "stall" {
					fmt.Fprintf(conn, "%s OK [CAPABILITY IMAP4rev1] logged in\r\n", tag)
				}
			case "EXAMINE":
				if mode == "missing-folder" {
					fmt.Fprintf(conn, "%s NO mailbox missing\r\n", tag)
				} else {
					fmt.Fprintf(conn, "* 0 EXISTS\r\n%s OK [READ-ONLY] examined\r\n", tag)
				}
			default:
				fmt.Fprintf(conn, "%s BAD unexpected command\r\n", tag)
			}
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	cfg := Config{Host: "127.0.0.1", Port: addr.Port, Username: "test", Password: "test-password", Folder: "INBOX"}
	dialer := &tls.Dialer{Config: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	return cfg, dialer.DialContext, done
}

func TestCheckConnectionReadOnly(t *testing.T) {
	for _, mode := range []string{"success", "reject", "missing-folder", "stall"} {
		t.Run(mode, func(t *testing.T) {
			cfg, dial, done := imapDiagnosticServer(t, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if mode == "stall" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			err := checkConnection(ctx, cfg, dial)
			if (err == nil) != (mode == "success") {
				t.Fatalf("mode %s: %v", mode, err)
			}
			commands := <-done
			if !slices.Contains(commands, "LOGIN") {
				t.Fatalf("did not authenticate: %v", commands)
			}
			if mode == "success" && !slices.Contains(commands, "EXAMINE") {
				t.Fatalf("did not examine folder: %v", commands)
			}
			if (mode == "reject" || mode == "stall") && slices.Contains(commands, "EXAMINE") {
				t.Fatalf("examined folder without authenticating: %v", commands)
			}
			for _, command := range commands {
				if command != "CAPABILITY" && command != "LOGIN" && command != "EXAMINE" {
					t.Fatalf("unexpected command %q", command)
				}
			}
		})
	}
}

func TestCheckConnectionRejectsUntrustedTLS(t *testing.T) {
	cfg, _, done := imapDiagnosticServer(t, "success")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := CheckConnection(ctx, cfg); err == nil {
		t.Fatal("untrusted TLS certificate accepted")
	}
	if commands := <-done; len(commands) != 0 {
		t.Fatalf("sent credentials before verifying TLS: %v", commands)
	}
}
