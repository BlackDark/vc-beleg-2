package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// identity is what the mock provider puts into the next ID token.
type identity struct {
	Subject string
	Name    string
	Groups  []string
}

// mockProvider is a minimal OpenID provider: discovery, authorize, token
// and JWKS. It checks PKCE and client credentials like a real provider and
// signs ID tokens with RS256.
type mockProvider struct {
	*httptest.Server
	t        *testing.T
	key      *rsa.PrivateKey
	clientID string
	secret   string

	mu       sync.Mutex
	next     identity
	badNonce bool // sign the next token with a nonce the app did not send
	codes    map[string]grant
}

type grant struct {
	challenge, nonce, redirect string
	id                         identity
}

func newMockProvider(t *testing.T) *mockProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &mockProvider{t: t, key: key, clientID: "mahlzeit", secret: "client-secret", codes: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	mux.HandleFunc("GET /jwks", p.jwks)
	p.Server = httptest.NewServer(mux)
	t.Cleanup(p.Close)
	return p
}

func (p *mockProvider) as(id identity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = id
}

func (p *mockProvider) config() *OIDCConfig {
	return &OIDCConfig{Issuer: p.URL, ClientID: p.clientID, ClientSecret: p.secret, Policy: app.OIDCPolicy{AllowedGroups: []string{"mahlzeit"}}}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (p *mockProvider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.URL,
		"authorization_endpoint":                p.URL + "/authorize",
		"token_endpoint":                        p.URL + "/token",
		"jwks_uri":                              p.URL + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (p *mockProvider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != p.clientID || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("nonce") == "" || q.Get("state") == "" {
		http.Error(w, "invalid authorization request", http.StatusBadRequest)
		return
	}
	code := base64.RawURLEncoding.EncodeToString(randomKey(16))
	p.mu.Lock()
	p.codes[code] = grant{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), redirect: q.Get("redirect_uri"), id: p.next}
	p.mu.Unlock()
	back, _ := url.Parse(q.Get("redirect_uri"))
	back.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (p *mockProvider) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	p.mu.Lock()
	g, found := p.codes[r.PostForm.Get("code")]
	delete(p.codes, r.PostForm.Get("code"))
	badNonce := p.badNonce
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	switch {
	case id != p.clientID || secret != p.secret:
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		return
	case !found || g.redirect != r.PostForm.Get("redirect_uri") || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}
	nonce := g.nonce
	if badNonce {
		nonce = "forged"
	}
	now := time.Now()
	claims := map[string]any{
		"iss": p.URL, "sub": g.id.Subject, "aud": []string{p.clientID}, "azp": p.clientID,
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "nonce": nonce,
		"name": g.id.Name, "email": g.id.Subject + "@example.test", "groups": g.id.Groups,
	}
	writeJSON(w, map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 300, "id_token": p.sign(claims)})
}

func (p *mockProvider) sign(claims map[string]any) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: p.key, KeyID: "k1"}}, nil)
	if err != nil {
		p.t.Fatal(err)
	}
	payload, _ := json.Marshal(claims)
	jws, err := signer.Sign(payload)
	if err != nil {
		p.t.Fatal(err)
	}
	out, _ := jws.CompactSerialize()
	return out
}

func (p *mockProvider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &p.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

// oidcLogin runs the browser part of the code flow: it follows redirects
// through the provider and the callback and returns where the app finally
// sends the browser.
func (c *client) oidcLogin(path string) string {
	c.t.Helper()
	hc := *c.http
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	next := c.base + path
	for range 5 {
		resp, err := hc.Get(next)
		if err != nil {
			c.t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			return resp.Status
		}
		loc, _ := resp.Request.URL.Parse(resp.Header.Get("Location"))
		if loc.Host == strings.TrimPrefix(c.base, "http://") && !strings.HasPrefix(loc.Path, "/auth/") {
			return loc.RequestURI()
		}
		next = loc.String()
	}
	c.t.Fatal("too many redirects")
	return ""
}

type sessionUser struct {
	User *struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
	NeedsSetup bool `json:"needsSetup"`
}

func (c *client) session() sessionUser {
	c.t.Helper()
	var s sessionUser
	if st := c.do("GET", "/api/session", nil, &s); st != 200 {
		c.t.Fatalf("session: %d", st)
	}
	return s
}

// TestOIDC drives the code flow against a mock provider: only allowed
// identities get in, the first one creates the account, a second allowed
// identity cannot take it over, forged nonce and state are rejected.
func TestOIDC(t *testing.T) {
	idp := newMockProvider(t)
	s := newServer(t, func(c *Config) { c.OIDC = idp.config() })

	t.Run("identity outside the allowed groups is refused", func(t *testing.T) {
		idp.as(identity{Subject: "stranger", Name: "Stranger", Groups: []string{"other"}})
		c := s.client(t)
		if got := c.oidcLogin("/auth/oidc/login"); got != "/login?error=oidc.not_allowed" {
			t.Fatalf("redirect: %s", got)
		}
		if c.session().User != nil {
			t.Fatal("session created")
		}
	})

	t.Run("allowed identity creates the account and logs in again", func(t *testing.T) {
		idp.as(identity{Subject: "alice", Name: "Alice", Groups: []string{"mahlzeit"}})
		c := s.client(t)
		if got := c.oidcLogin("/auth/oidc/login"); got != "/" {
			t.Fatalf("redirect: %s", got)
		}
		first := c.session()
		if first.User == nil || first.User.Name != "Alice" {
			t.Fatalf("session after first login: %+v", first.User)
		}
		if s.client(t).session().NeedsSetup {
			t.Error("setup still offered after the account exists")
		}
		c = s.client(t)
		c.oidcLogin("/auth/oidc/login")
		if again := c.session(); again.User == nil || again.User.ID != first.User.ID {
			t.Fatalf("second login: %+v", again.User)
		}
	})

	t.Run("a second allowed identity cannot take over the account", func(t *testing.T) {
		idp.as(identity{Subject: "mallory", Name: "Mallory", Groups: []string{"mahlzeit"}})
		if got := s.client(t).oidcLogin("/auth/oidc/login"); got != "/login?error=oidc.other_identity" {
			t.Fatalf("redirect: %s", got)
		}
	})

	t.Run("an allowed identity is linked to the password account", func(t *testing.T) {
		fresh := newServer(t, func(c *Config) { c.OIDC = idp.config() })
		bob := fresh.client(t)
		if st := bob.do("POST", "/api/setup", map[string]any{"token": "setup-token", "username": "bob", "name": "Bob", "password": "correct horse battery"}, nil); st != 200 {
			t.Fatalf("setup: %d", st)
		}
		idp.as(identity{Subject: "bob-sub", Name: "Bob (IdP)", Groups: []string{"mahlzeit"}})
		c := fresh.client(t)
		if got := c.oidcLogin("/auth/oidc/login"); got != "/" {
			t.Fatalf("login: %s", got)
		}
		if u := c.session().User; u == nil || u.Name != "Bob" {
			t.Fatalf("linked login: %+v", u)
		}
	})

	t.Run("forged nonce is rejected", func(t *testing.T) {
		idp.as(identity{Subject: "alice", Name: "Alice", Groups: []string{"mahlzeit"}})
		idp.mu.Lock()
		idp.badNonce = true
		idp.mu.Unlock()
		defer func() { idp.mu.Lock(); idp.badNonce = false; idp.mu.Unlock() }()
		c := s.client(t)
		if got := c.oidcLogin("/auth/oidc/login"); got != "/login?error=oidc.failed" {
			t.Fatalf("redirect: %s", got)
		}
		if c.session().User != nil {
			t.Fatal("session created with a forged nonce")
		}
	})

	t.Run("callback with a foreign state is rejected", func(t *testing.T) {
		c := s.client(t)
		resp, err := (&http.Client{Jar: c.http.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(c.base + "/auth/oidc/login")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got := c.oidcLogin("/auth/oidc/callback?code=stolen&state=forged"); got != "/login?error=oidc.failed" {
			t.Fatalf("redirect: %s", got)
		}
		if c.session().User != nil {
			t.Fatal("session created without a matching login")
		}
	})
}
