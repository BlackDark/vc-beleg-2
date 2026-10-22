package api

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func pngBytes() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 60)))
	return b.Bytes()
}

func TestPasswordLoginAndAccess(t *testing.T) {
	s := newServer(t)
	anon := s.client(t)
	if st := anon.do("GET", "/api/receipts", nil, nil); st != 401 {
		t.Fatalf("anonymous list: %d", st)
	}
	if !anon.session().NeedsSetup {
		t.Fatal("setup not offered")
	}
	if st := anon.do("POST", "/api/setup", map[string]any{"token": "wrong", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); st != 422 {
		t.Fatalf("setup with wrong token: %d", st)
	}
	if st := anon.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); st != 200 {
		t.Fatalf("setup: %d", st)
	}
	if st := s.client(t).do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "eve", "name": "Eve", "password": "correct horse battery"}, nil); st != 409 {
		t.Fatalf("second setup: %d", st)
	}

	c := s.client(t)
	for range 5 {
		c.do("POST", "/api/login", map[string]any{"username": "max", "password": "wrong password"}, nil)
	}
	if st := c.do("POST", "/api/login", map[string]any{"username": "max", "password": "correct horse battery"}, nil); st != 429 {
		t.Fatalf("login after 5 failures: %d", st)
	}

	// The setup client is logged in: upload, read back, check.
	var r struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if st := anon.do("POST", "/api/receipts?filename=bon.png", pngBytes(), &r); st != 201 || r.Status != "pending" {
		t.Fatalf("upload: %d %+v", st, r)
	}
	if st := anon.do("POST", "/api/receipts/1/check", map[string]any{"date": "2026-10-05", "total": 850, "merchant": "REWE", "note": ""}, &r); st != 200 || r.Status != "checked" {
		t.Fatalf("check: %d %+v", st, r)
	}
	var m struct {
		Result struct {
			Totals struct {
				Allowance int `json:"allowance"`
			} `json:"totals"`
		} `json:"result"`
	}
	if st := anon.do("GET", "/api/months/2026-10", nil, &m); st != 200 || m.Result.Totals.Allowance != 767 {
		t.Fatalf("month: %d %+v", st, m)
	}
	if st := anon.do("POST", "/api/logout", nil, nil); st != 200 {
		t.Fatalf("logout: %d", st)
	}
	if st := anon.do("GET", "/api/months/2026-10", nil, nil); st != 401 {
		t.Fatalf("after logout: %d", st)
	}
}
