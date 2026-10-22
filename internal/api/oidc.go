package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// OIDCConfig configures login via an OpenID Connect provider such as Pocket ID.
type OIDCConfig struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	Label         string
	Scopes        []string
	GroupsClaim   string
	Policy        app.OIDCPolicy
}

// oidcProvider performs discovery lazily so the app starts even when the
// provider is temporarily unreachable.
type oidcProvider struct {
	cfg      *OIDCConfig
	redirect string
	secure   bool
	label    string
	cookies  *httphelper.CookieHandler
	log      *slog.Logger

	mu    sync.Mutex
	party rp.RelyingParty
}

type nonceCtx struct{}

const nonceCookie = "mahlzeit_oidc_nonce"

func randomKey(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func newOIDCProvider(_ context.Context, cfg *OIDCConfig, baseURL string, secure bool, log *slog.Logger) (*oidcProvider, error) {
	opts := []httphelper.CookieHandlerOpt{httphelper.WithSameSite(http.SameSiteLaxMode), httphelper.WithMaxAge(600)}
	if !secure {
		opts = append(opts, httphelper.WithUnsecure())
	}
	label := cfg.Label
	if label == "" {
		label = "Single Sign-On"
	}
	return &oidcProvider{
		cfg:      cfg,
		redirect: strings.TrimRight(baseURL, "/") + "/auth/oidc/callback",
		secure:   secure,
		label:    label,
		cookies:  httphelper.NewCookieHandler(randomKey(32), randomKey(32), opts...),
		log:      log,
	}, nil
}

func (p *oidcProvider) relyingParty(ctx context.Context) (rp.RelyingParty, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.party != nil {
		return p.party, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scopes := p.cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail, "groups"}
	}
	party, err := rp.NewRelyingPartyOIDC(ctx, p.cfg.Issuer, p.cfg.ClientID, p.cfg.ClientSecret, p.redirect, scopes,
		rp.WithPKCE(p.cookies),
		rp.WithVerifierOpts(rp.WithNonce(func(ctx context.Context) string {
			v, _ := ctx.Value(nonceCtx{}).(string)
			return v
		})),
		rp.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, errorType, errorDesc, _ string) {
			p.callbackFailed(w, r, errorType+": "+errorDesc)
		}),
		rp.WithUnauthorizedHandler(func(w http.ResponseWriter, r *http.Request, desc, _ string) {
			p.callbackFailed(w, r, desc)
		}),
	)
	if err != nil {
		return nil, err
	}
	p.party = party
	return party, nil
}

// callbackFailed sends the browser back to the login page instead of the
// library's plain-text error (state mismatch, failed exchange or token check).
func (p *oidcProvider) callbackFailed(w http.ResponseWriter, r *http.Request, reason string) {
	p.log.Warn("oidc callback failed", "reason", reason)
	http.Redirect(w, r, "/login?error=oidc.failed", http.StatusFound)
}

func (s *Server) shortCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/auth/oidc", HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

// oidcLogin starts Authorization Code + PKCE with state and nonce.
func (s *Server) oidcLogin(w http.ResponseWriter, r *http.Request) {
	party, err := s.oidc.relyingParty(r.Context())
	if err != nil {
		s.log.Error("oidc discovery failed", "err", err)
		http.Redirect(w, r, "/login?error=oidc.unavailable", http.StatusFound)
		return
	}
	nonce := base64.RawURLEncoding.EncodeToString(randomKey(24))
	state := base64.RawURLEncoding.EncodeToString(randomKey(24))
	http.SetCookie(w, s.shortCookie(nonceCookie, nonce, 600))
	rp.AuthURLHandler(func() string { return state }, party, rp.WithURLParam("nonce", nonce))(w, r)
}

// oidcCallback exchanges the code, verifies the ID token (issuer, audience,
// signature, expiry, nonce) and logs in.
func (s *Server) oidcCallback(w http.ResponseWriter, r *http.Request) {
	party, err := s.oidc.relyingParty(r.Context())
	if err != nil {
		http.Redirect(w, r, "/login?error=oidc.unavailable", http.StatusFound)
		return
	}
	nonce, _ := r.Cookie(nonceCookie)
	http.SetCookie(w, s.shortCookie(nonceCookie, "", -1))
	if nonce == nil || nonce.Value == "" {
		http.Redirect(w, r, "/login?error=oidc.failed", http.StatusFound)
		return
	}
	ctx := context.WithValue(r.Context(), nonceCtx{}, nonce.Value)
	callback := func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], _ string, _ rp.RelyingParty) {
		claims := tokens.IDTokenClaims
		id := app.OIDCIdentity{Issuer: claims.Issuer, Subject: claims.Subject, Name: claims.Name, Email: claims.Email, Groups: groups(claims.Claims, s.oidc.cfg.GroupsClaim)}
		if id.Name == "" {
			id.Name = claims.PreferredUsername
		}
		u, err := s.app.OIDCLogin(r.Context(), id, s.oidc.cfg.Policy)
		if err != nil {
			http.Redirect(w, r, "/login?error="+url.QueryEscape(errorCode(err)), http.StatusFound)
			return
		}
		token, err := s.app.CreateSession(r.Context(), u.ID, "oidc")
		if err != nil {
			http.Redirect(w, r, "/login?error=internal", http.StatusFound)
			return
		}
		c := s.cookie(token, int(app.SessionTTL.Seconds()))
		http.SetCookie(w, &c)
		http.Redirect(w, r, "/", http.StatusFound)
	}
	rp.CodeExchangeHandler(callback, party)(w, r.WithContext(ctx))
}

func errorCode(err error) string {
	if ae, ok := err.(*app.Error); ok {
		return ae.Code
	}
	return "oidc.failed"
}

// groups reads a string-array claim.
func groups(claims map[string]any, name string) []string {
	if name == "" {
		name = "groups"
	}
	raw, ok := claims[name].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, g := range raw {
		if s, ok := g.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
