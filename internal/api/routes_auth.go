package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark/vc-beleg-2/internal/app"
)

// SessionInfo tells the UI how to log in and who is logged in.
type SessionInfo struct {
	User            *app.User `json:"user,omitempty" doc:"absent when not logged in"`
	NeedsSetup      bool      `json:"needsSetup" doc:"no account exists and password login is on, so /setup can create one"`
	PasswordEnabled bool      `json:"passwordEnabled"`
	OIDCEnabled     bool      `json:"oidcEnabled"`
	OIDCLabel       string    `json:"oidcLabel"`
	Version         string    `json:"version"`
}

type loginBody struct {
	Username string `json:"username" minLength:"1" maxLength:"100"`
	Password string `json:"password" minLength:"1" maxLength:"500"`
}

type setupBody struct {
	Token    string `json:"token" minLength:"1"`
	Username string `json:"username" minLength:"1" maxLength:"100"`
	Name     string `json:"name" minLength:"1" maxLength:"200"`
	Password string `json:"password" minLength:"10" maxLength:"500"`
}

type sessionOut struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      SessionInfo
}

func (s *Server) sessionInfo(ctx context.Context, u *app.User) (SessionInfo, error) {
	info := SessionInfo{User: u, PasswordEnabled: s.cfg.PasswordEnabled, OIDCEnabled: s.oidc != nil, Version: s.cfg.Version}
	if s.oidc != nil {
		info.OIDCLabel = s.oidc.label
	}
	needs, err := s.app.NeedsSetup(ctx)
	// Setup creates a password account, so it is only offered with password
	// login; OIDC-only instances create the account on the first allowed login.
	info.NeedsSetup = needs && s.cfg.PasswordEnabled
	return info, err
}

func (s *Server) registerAuth(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "getSession", Method: http.MethodGet, Path: "/api/session", Tags: []string{"auth"}, Summary: "Current session and login options"},
		func(ctx context.Context, _ *empty) (*body[SessionInfo], error) {
			info, err := s.sessionInfo(ctx, userFrom(ctx))
			if err != nil {
				return nil, s.fail(ctx, err)
			}
			return &body[SessionInfo]{Body: info}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "setup", Method: http.MethodPost, Path: "/api/setup", Tags: []string{"auth"}, Summary: "Create the account (single use)"},
		func(ctx context.Context, in *body[setupBody]) (*sessionOut, error) {
			if !s.cfg.PasswordEnabled {
				return nil, &ErrorModel{Status: 403, Code: "setup.password_disabled", Message: "password login is disabled"}
			}
			u, err := s.app.Setup(ctx, in.Body.Token, s.cfg.SetupToken, in.Body.Username, in.Body.Name, in.Body.Password)
			if err != nil {
				return nil, s.fail(ctx, err)
			}
			return s.startSession(ctx, u, "password")
		})

	huma.Register(api, huma.Operation{OperationID: "login", Method: http.MethodPost, Path: "/api/login", Tags: []string{"auth"}, Summary: "Password login"},
		func(ctx context.Context, in *body[loginBody]) (*sessionOut, error) {
			if !s.cfg.PasswordEnabled {
				return nil, &ErrorModel{Status: 403, Code: "login.password_disabled", Message: "password login is disabled"}
			}
			client, _ := ctx.Value(clientKey).(string)
			u, err := s.app.PasswordLogin(ctx, in.Body.Username, in.Body.Password, client)
			if err != nil {
				return nil, s.fail(ctx, err)
			}
			return s.startSession(ctx, u, "password")
		})

	huma.Register(api, huma.Operation{OperationID: "logout", Method: http.MethodPost, Path: "/api/logout", Tags: []string{"auth"}, Summary: "End the session"},
		func(ctx context.Context, _ *empty) (*sessionOut, error) {
			if token, ok := ctx.Value(tokenKey).(string); ok {
				if err := s.app.EndSession(ctx, token); err != nil {
					return nil, s.fail(ctx, err)
				}
			}
			info, err := s.sessionInfo(ctx, nil)
			if err != nil {
				return nil, s.fail(ctx, err)
			}
			return &sessionOut{SetCookie: s.cookie("", -1), Body: info}, nil
		})
}

func (s *Server) startSession(ctx context.Context, u *app.User, method string) (*sessionOut, error) {
	token, err := s.app.CreateSession(ctx, u.ID, method)
	if err != nil {
		return nil, s.fail(ctx, err)
	}
	info, err := s.sessionInfo(ctx, u)
	if err != nil {
		return nil, s.fail(ctx, err)
	}
	return &sessionOut{SetCookie: s.cookie(token, int(app.SessionTTL.Seconds())), Body: info}, nil
}
