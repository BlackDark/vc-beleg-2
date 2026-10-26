package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/rates"
)

type passwordBody struct {
	Current string `json:"current"`
	Next    string `json:"next" minLength:"10" maxLength:"500"`
}

func (s *Server) registerProfile(api huma.API) {
	tags := []string{"profile"}
	authed(s, api, huma.Operation{OperationID: "getReaderStatus", Method: http.MethodGet, Path: "/api/reader-status", Tags: tags, Summary: "Receipt reader configuration without credentials"},
		func(ctx context.Context, u *app.User, _ *empty) (*body[app.ReaderInfo], error) {
			return wrap(s.app.Cfg.ReaderInfo, nil)
		})
	authed(s, api, huma.Operation{OperationID: "getMe", Method: http.MethodGet, Path: "/api/me", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[*app.User], error) {
			return &body[*app.User]{Body: u}, nil
		})
	authed(s, api, huma.Operation{OperationID: "updateMe", Method: http.MethodPut, Path: "/api/me", Tags: tags},
		func(ctx context.Context, u *app.User, in *body[app.ProfileInput]) (*body[*app.User], error) {
			return wrap(s.app.UpdateProfile(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "changePassword", Method: http.MethodPost, Path: "/api/me/password", Tags: tags, DefaultStatus: 204},
		func(ctx context.Context, u *app.User, in *body[passwordBody]) (*empty, error) {
			return &empty{}, s.app.ChangePassword(ctx, u, in.Body.Current, in.Body.Next)
		})
	authed(s, api, huma.Operation{OperationID: "getSettings", Method: http.MethodGet, Path: "/api/settings", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[*app.Settings], error) {
			return wrap(s.app.GetSettings(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "saveSettings", Method: http.MethodPut, Path: "/api/settings", Tags: tags},
		func(ctx context.Context, u *app.User, in *body[app.Settings]) (*body[*app.Settings], error) {
			return wrap(s.app.SaveSettings(ctx, u, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "listRates", Method: http.MethodGet, Path: "/api/rates", Tags: tags, Summary: "Benefit values per year"},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]rates.Values], error) {
			return wrap(s.app.Rates.Sorted(), nil)
		})
	authed(s, api, huma.Operation{OperationID: "getMailStatus", Method: http.MethodGet, Path: "/api/mail-status", Tags: tags, Summary: "State of the mail import"},
		func(ctx context.Context, u *app.User, _ *empty) (*body[app.MailStatus], error) {
			return wrap(s.app.MailStatus(), nil)
		})
	authed(s, api, huma.Operation{OperationID: "getFolderStatus", Method: http.MethodGet, Path: "/api/folder-status", Tags: tags, Summary: "State of the folder import"},
		func(ctx context.Context, u *app.User, _ *empty) (*body[app.FolderStatus], error) {
			return wrap(s.app.FolderStatus(), nil)
		})
	authed(s, api, huma.Operation{OperationID: "getImportLog", Method: http.MethodGet, Path: "/api/import-log", Tags: tags, Summary: "Latest import and receipt reading events"},
		func(ctx context.Context, u *app.User, in *importLogQuery) (*body[[]app.LogEntry], error) {
			return wrap(s.app.ImportLog(ctx, u, in.Limit))
		})
}

type importLogQuery struct {
	Limit int `query:"limit" minimum:"1" maximum:"200" default:"50"`
}
