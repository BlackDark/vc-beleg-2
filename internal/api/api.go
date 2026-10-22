// Package api exposes the domain services over HTTP: Huma operations under
// /api, OIDC redirects under /auth, health checks and the embedded SPA.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// SessionCookie is the name of the session cookie.
const SessionCookie = "mahlzeit_session"

// Config configures the HTTP layer.
type Config struct {
	BaseURL         string // public URL, e.g. https://mahlzeit.example.com
	SecureCookies   bool
	SetupToken      string
	PasswordEnabled bool
	OIDC            *OIDCConfig
	Version         string
}

// Server wires handlers to the services.
type Server struct {
	app  *app.App
	cfg  Config
	log  *slog.Logger
	oidc *oidcProvider
}

// ErrorModel is the API error body. Code is a stable key the UI translates.
type ErrorModel struct {
	Status  int                 `json:"status"`
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Params  map[string]string   `json:"params,omitempty"`
	Errors  []*huma.ErrorDetail `json:"errors,omitempty"`
}

func (e *ErrorModel) Error() string  { return e.Message }
func (e *ErrorModel) GetStatus() int { return e.Status }

func init() {
	// Go handlers always return empty slices, never nil, so arrays are not
	// nullable in the generated TypeScript types.
	huma.DefaultArrayNullable = false
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		code := "http_error"
		switch status {
		case 400, 422:
			code = "validation"
		case 401:
			code = "unauthorized"
		case 404:
			code = "not_found"
		case 413:
			code = "upload.too_large"
		}
		m := &ErrorModel{Status: status, Code: code, Message: msg}
		for _, e := range errs {
			var d *huma.ErrorDetail
			if errors.As(e, &d) {
				m.Errors = append(m.Errors, d)
			} else if e != nil {
				m.Errors = append(m.Errors, &huma.ErrorDetail{Message: e.Error()})
			}
		}
		return m
	}
}

// fail converts domain errors into API errors and hides internal ones.
func (s *Server) fail(ctx context.Context, err error) error {
	var ae *app.Error
	if errors.As(err, &ae) {
		return &ErrorModel{Status: ae.Status, Code: ae.Code, Message: ae.Message, Params: ae.Params}
	}
	s.log.ErrorContext(ctx, "request failed", "err", err)
	return &ErrorModel{Status: 500, Code: "internal", Message: "internal error"}
}

type ctxKey int

const (
	userKey ctxKey = iota
	tokenKey
	clientKey
)

func userFrom(ctx context.Context) *app.User {
	u, _ := ctx.Value(userKey).(*app.User)
	return u
}

// New builds the HTTP handler.
func New(a *app.App, cfg Config, log *slog.Logger, spa http.Handler) (http.Handler, error) {
	s := &Server{app: a, cfg: cfg, log: log}
	if cfg.OIDC != nil {
		p, err := newOIDCProvider(context.Background(), cfg.OIDC, cfg.BaseURL, cfg.SecureCookies, log)
		if err != nil {
			return nil, err
		}
		s.oidc = p
	}
	mux := http.NewServeMux()
	s.API(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	if s.oidc != nil {
		mux.HandleFunc("GET /auth/oidc/login", s.oidcLogin)
		mux.HandleFunc("GET /auth/oidc/callback", s.oidcCallback)
	}
	if spa != nil {
		mux.Handle("/", spa)
	}
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux)), nil
}

// API registers all operations on mux and returns the Huma API (also used
// to export the OpenAPI document).
func (s *Server) API(mux *http.ServeMux) huma.API {
	config := huma.DefaultConfig("Mahlzeit API", s.cfg.Version)
	config.Info.Description = "Meal allowance receipts and monthly reports."
	config.OpenAPIPath = "/api/openapi"
	config.CreateHooks = nil // no $schema links in responses
	config.DocsPath = ""
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{"session": {Type: "apiKey", In: "cookie", Name: SessionCookie}}
	api := humago.New(mux, config)
	api.UseMiddleware(s.authenticate)
	s.registerAuth(api)
	s.registerProfile(api)
	s.registerReceipts(api)
	s.registerMonths(api)
	return api
}

// NewForSpec builds the API without services, for OpenAPI generation.
func NewForSpec(version string) huma.API {
	s := &Server{cfg: Config{Version: version}, log: slog.Default()}
	return s.API(http.NewServeMux())
}

// authenticate resolves the session cookie into the request context.
func (s *Server) authenticate(ctx huma.Context, next func(huma.Context)) {
	ctx = huma.WithValue(ctx, clientKey, clientKeyOf(ctx.RemoteAddr()))
	if c, err := huma.ReadCookie(ctx, SessionCookie); err == nil && s.app != nil {
		if u, err := s.app.SessionUser(ctx.Context(), c.Value); err == nil {
			ctx = huma.WithValue(ctx, userKey, u)
			ctx = huma.WithValue(ctx, tokenKey, c.Value)
		}
	}
	next(ctx)
}

func clientKeyOf(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func (s *Server) cookie(token string, maxAge int) http.Cookie {
	return http.Cookie{Name: SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-src 'self' blob:; object-src 'self' blob:; base-uri 'none'; form-action 'self'; frame-ancestors 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// Generic input/output wrappers.
type (
	empty       struct{}
	body[T any] struct{ Body T }
	idIn        struct {
		ID int64 `path:"id"`
	}
	idBody[T any] struct {
		ID   int64 `path:"id"`
		Body T
	}
	reasonBody struct {
		Reason string `json:"reason" minLength:"1" maxLength:"1000"`
	}
)

// authed registers an operation that requires a logged-in user.
func authed[I, O any](s *Server, api huma.API, op huma.Operation, h func(ctx context.Context, u *app.User, in *I) (*O, error)) {
	op.Security = []map[string][]string{{"session": {}}}
	huma.Register(api, op, func(ctx context.Context, in *I) (*O, error) {
		u := userFrom(ctx)
		if u == nil {
			return nil, &ErrorModel{Status: 401, Code: "unauthorized", Message: "login required"}
		}
		out, err := h(ctx, u, in)
		if err != nil {
			return nil, s.fail(ctx, err)
		}
		return out, nil
	})
}

func wrap[T any](v T, err error) (*body[T], error) {
	if err != nil {
		return nil, err
	}
	return &body[T]{Body: v}, nil
}
