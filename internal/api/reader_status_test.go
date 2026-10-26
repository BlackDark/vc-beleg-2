package api

import "testing"

func TestReaderStatusRequiresLogin(t *testing.T) {
	s := newServer(t)
	if status := s.client(t).do("GET", "/api/reader-status", nil, nil); status != 401 {
		t.Fatalf("reader status anonymous: %d", status)
	}
}

func TestReaderStatusForAuthenticatedUser(t *testing.T) {
	s := newServer(t)
	c := s.client(t)
	if status := c.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "max", "name": "Max", "password": "correct horse battery"}, nil); status != 200 {
		t.Fatal(status)
	}
	var info map[string]any
	if status := c.do("GET", "/api/reader-status", nil, &info); status != 200 {
		t.Fatal(status)
	}
	if info["enabled"] != false {
		t.Fatalf("disabled reader: %v", info)
	}
	for _, key := range []string{"apiKey", "password", "token"} {
		if _, ok := info[key]; ok {
			t.Fatalf("credential field %s exposed", key)
		}
	}
}
