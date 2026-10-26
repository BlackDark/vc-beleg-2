package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

type testReader struct{ calls chan chan error }

func (r *testReader) Extract(ctx context.Context, _ string, _ []byte) (app.Extraction, error) {
	result := make(chan error, 1)
	select {
	case r.calls <- result:
	case <-ctx.Done():
		return app.Extraction{}, ctx.Err()
	}
	select {
	case err := <-result:
		return app.Extraction{Merchant: "Read"}, err
	case <-ctx.Done():
		return app.Extraction{}, ctx.Err()
	}
}

func TestExtractionStatusAndIdempotentRetry(t *testing.T) {
	s := newServer(t)
	reader := &testReader{calls: make(chan chan error, 4)}
	s.app.Extractor = reader
	c := s.client(t)
	if st := c.do("POST", "/api/receipts/1/extract", nil, nil); st != 401 {
		t.Fatalf("anonymous retry: %d", st)
	}
	if st := c.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); st != 200 {
		t.Fatalf("setup: %d", st)
	}
	var r app.Receipt
	if st := c.do("POST", "/api/receipts?filename=receipt.png", pngBytes(), &r); st != 201 || r.Extraction != "queued" {
		t.Fatalf("upload: %d %+v", st, r)
	}
	endpoint := fmt.Sprintf("/api/receipts/%d", r.ID)
	for range 3 {
		if st := c.do("POST", endpoint+"/extract", nil, &r); st != 200 || r.Extraction != "queued" {
			t.Fatalf("queued retry: %d %+v", st, r)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.app.RunExtraction(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("worker did not stop")
		}
	})
	select {
	case result := <-reader.calls:
		if st := c.do("GET", endpoint, nil, &r); st != 200 || r.Extraction != "processing" {
			t.Fatalf("processing: %d %+v", st, r)
		}
		if st := c.do("POST", endpoint+"/extract", nil, &r); st != 200 || r.Extraction != "processing" {
			t.Fatalf("active retry: %d %+v", st, r)
		}
		result <- errors.New("provider unavailable")
	case <-time.After(5 * time.Second):
		t.Fatal("reading did not start")
	}
	waitAPIExtraction(t, c, endpoint, "failed")
	if st := c.do("POST", endpoint+"/extract", nil, &r); st != 200 {
		t.Fatalf("failed retry: %d", st)
	}
	select {
	case result := <-reader.calls:
		result <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("retry did not start")
	}
	waitAPIExtraction(t, c, endpoint, "done")
	if st := c.do("POST", endpoint+"/extract", nil, nil); st != 422 {
		t.Fatalf("successful reading retried: %d", st)
	}
	if st := c.do("POST", "/api/receipts/999/extract", nil, nil); st != 404 {
		t.Fatalf("missing receipt retry: %d", st)
	}
	select {
	case <-reader.calls:
		t.Fatal("retry produced duplicate job")
	default:
	}
}

func waitAPIExtraction(t *testing.T, c *client, endpoint, state string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var r app.Receipt
		if st := c.do("GET", endpoint, nil, &r); st != 200 {
			t.Fatalf("receipt: %d", st)
		}
		if r.Extraction == state {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("receipt never reached %s", state)
}
