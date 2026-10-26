package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

func TestConnectionCheckRequiresLogin(t *testing.T) {
	s := newServer(t)
	if status := s.client(t).do("POST", "/api/services/reader/check", map[string]any{}, nil); status != 401 {
		t.Fatalf("anonymous check status=%d", status)
	}
}

func TestConnectionCheckResults(t *testing.T) {
	s := newServer(t)
	c := s.client(t)
	if status := c.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); status != 200 {
		t.Fatal(status)
	}
	if status := c.do("POST", "/api/services/reader/check", nil, nil); status != 422 {
		t.Fatalf("disabled service status=%d", status)
	}
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"success", nil, ""}, {"failure", errors.New("credential-secret remote message"), "failed"}, {"timeout", context.DeadlineExceeded, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.app.ConnectionChecks["reader"] = func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("unbounded diagnostic")
				}
				return tc.err
			}
			var result app.ConnectionResult
			if status := c.do("POST", "/api/services/reader/check", nil, &result); status != 200 {
				t.Fatal(status)
			}
			if result.OK != (tc.err == nil) || result.ErrorCode != tc.code || result.DurationMS < 0 || strings.Contains(result.ErrorCode, "secret") {
				t.Fatalf("unsafe/incorrect result: %+v", result)
			}
		})
	}
	if status := c.do("POST", "/api/services/unknown/check", nil, nil); status != 422 {
		t.Fatalf("unknownservice status=%d", status)
	}
}
