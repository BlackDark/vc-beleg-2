package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/folderimport"
)

func TestFolderConnectionCheckOnlyLists(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		user, password, ok := r.BasicAuth()
		if r.Method != "PROPFIND" || r.Header.Get("Depth") != "1" || !ok || user != "user" || password != "password" {
			t.Errorf("unexpected connection request %s", r.Method)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		io.WriteString(w, `<d:multistatus xmlns:d="DAV:"><d:response><d:href>/receipt.png</d:href></d:response></d:multistatus>`)
	}))
	defer srv.Close()
	a := &app.App{ConnectionChecks: map[string]func(context.Context) error{}}
	cfg := &Config{Folder: &folderimport.Config{URL: srv.URL, Username: "user", Password: "password"}}
	if err := configureConnections(cfg, a, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.ConnectionChecks["folder"](context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("connection check made %d requests, expected listing only", calls)
	}
	if a.ConnectionChecks["reader"] != nil || a.ConnectionChecks["mail"] != nil {
		t.Fatal("unconfigured service enabled")
	}
}
