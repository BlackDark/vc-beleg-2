package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/calc"
)

type receiptListQuery struct {
	Month  string `query:"month" pattern:"^([0-9]{4}-[0-9]{2})?$" doc:"YYYY-MM; undated receipts are always included"`
	Status string `query:"status" enum:"pending,checked,"`
}

type uploadIn struct {
	Filename string `query:"filename" maxLength:"255"`
	Note     string `query:"note" maxLength:"500"`
	RawBody  []byte `contentType:"application/octet-stream"`
}

type fileOut struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               []byte
}

func attachment(name string, inline bool) string {
	kind := "attachment"
	if inline {
		kind = "inline"
	}
	return fmt.Sprintf("%s; filename*=UTF-8''%s", kind, url.PathEscape(name))
}

func (s *Server) registerReceipts(api huma.API) {
	tags := []string{"receipts"}
	authed(s, api, huma.Operation{OperationID: "listReceipts", Method: http.MethodGet, Path: "/api/receipts", Tags: tags},
		func(ctx context.Context, u *app.User, in *receiptListQuery) (*body[[]*app.Receipt], error) {
			return wrap(s.app.ListReceipts(ctx, u, in.Month, in.Status))
		})
	authed(s, api, huma.Operation{OperationID: "uploadReceipt", Method: http.MethodPost, Path: "/api/receipts", Tags: tags, DefaultStatus: 201, MaxBodyBytes: 26 << 20,
		Summary: "Upload a receipt (JPEG, PNG or PDF); it starts as pending"},
		func(ctx context.Context, u *app.User, in *uploadIn) (*body[*app.Receipt], error) {
			return wrap(s.app.Import(ctx, u, "upload", in.Filename, in.RawBody, in.Note, nil))
		})
	authed(s, api, huma.Operation{OperationID: "getReceipt", Method: http.MethodGet, Path: "/api/receipts/{id}", Tags: tags},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Receipt], error) {
			return wrap(s.app.GetReceipt(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "updateReceipt", Method: http.MethodPut, Path: "/api/receipts/{id}", Tags: tags, Summary: "Save corrections without changing the status"},
		func(ctx context.Context, u *app.User, in *idBody[app.ReceiptInput]) (*body[*app.Receipt], error) {
			return wrap(s.app.UpdateReceipt(ctx, u, in.ID, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "checkReceipt", Method: http.MethodPost, Path: "/api/receipts/{id}/check", Tags: tags, Summary: "Confirm the values; the receipt counts from now on"},
		func(ctx context.Context, u *app.User, in *idBody[app.ReceiptInput]) (*body[*app.Receipt], error) {
			return wrap(s.app.CheckReceipt(ctx, u, in.ID, in.Body))
		})
	authed(s, api, huma.Operation{OperationID: "uncheckReceipt", Method: http.MethodPost, Path: "/api/receipts/{id}/uncheck", Tags: tags},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Receipt], error) {
			return wrap(s.app.UncheckReceipt(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "retryExtraction", Method: http.MethodPost, Path: "/api/receipts/{id}/extract", Tags: tags, Summary: "Read the receipt again"},
		func(ctx context.Context, u *app.User, in *idIn) (*body[*app.Receipt], error) {
			return wrap(s.app.RetryExtraction(ctx, u, in.ID))
		})
	authed(s, api, huma.Operation{OperationID: "deleteReceipt", Method: http.MethodDelete, Path: "/api/receipts/{id}", Tags: tags, DefaultStatus: 204, Summary: "Delete a receipt that was never checked"},
		func(ctx context.Context, u *app.User, in *idIn) (*empty, error) {
			return &empty{}, s.app.DeleteReceipt(ctx, u, in.ID)
		})
	file := func(preview bool) func(ctx context.Context, u *app.User, in *idIn) (*fileOut, error) {
		return func(ctx context.Context, u *app.User, in *idIn) (*fileOut, error) {
			b, mime, name, err := s.app.ReceiptFile(ctx, u, in.ID, preview)
			if err != nil {
				return nil, err
			}
			return &fileOut{ContentType: mime, ContentDisposition: attachment(name, true), CacheControl: "private, max-age=3600", Body: b}, nil
		}
	}
	authed(s, api, huma.Operation{OperationID: "getReceiptFile", Method: http.MethodGet, Path: "/api/receipts/{id}/file", Tags: tags, Summary: "Original bytes"}, file(false))
	authed(s, api, huma.Operation{OperationID: "getReceiptPreview", Method: http.MethodGet, Path: "/api/receipts/{id}/preview", Tags: tags, Summary: "PNG preview of the first page, without EXIF"}, file(true))
}

type monthIn struct {
	Month string `path:"month" pattern:"^[0-9]{4}-[0-9]{2}$"`
}

type monthBody[T any] struct {
	Month string `path:"month" pattern:"^[0-9]{4}-[0-9]{2}$"`
	Body  T
}

type combineBody struct {
	Date     string `json:"date" format:"date"`
	Combined bool   `json:"combined"`
}

type mealBody struct {
	Date string    `json:"date" format:"date"`
	Meal calc.Meal `json:"meal" enum:"breakfast,lunch,dinner"`
}

type paidBody struct {
	PaidOn string `json:"paidOn" doc:"YYYY-MM-DD; empty clears it" maxLength:"10"`
}

func (s *Server) registerMonths(api huma.API) {
	tags := []string{"months"}
	authed(s, api, huma.Operation{OperationID: "listMonths", Method: http.MethodGet, Path: "/api/months", Tags: tags},
		func(ctx context.Context, u *app.User, _ *empty) (*body[[]app.MonthSummary], error) {
			return wrap(s.app.ListMonths(ctx, u))
		})
	authed(s, api, huma.Operation{OperationID: "getMonth", Method: http.MethodGet, Path: "/api/months/{month}", Tags: tags},
		func(ctx context.Context, u *app.User, in *monthIn) (*body[*app.Month], error) {
			return wrap(s.app.GetMonth(ctx, u, in.Month))
		})
	authed(s, api, huma.Operation{OperationID: "combineDay", Method: http.MethodPost, Path: "/api/days/combine", Tags: tags, Summary: "Combine the receipts of a date into one meal, or undo it"},
		func(ctx context.Context, u *app.User, in *body[combineBody]) (*body[*app.Month], error) {
			return wrap(s.app.SetCombined(ctx, u, in.Body.Date, in.Body.Combined))
		})
	authed(s, api, huma.Operation{OperationID: "setDayMeal", Method: http.MethodPost, Path: "/api/days/meal", Tags: tags, Summary: "Set the meal a date's allowance is for"},
		func(ctx context.Context, u *app.User, in *body[mealBody]) (*body[*app.Month], error) {
			return wrap(s.app.SetMeal(ctx, u, in.Body.Date, in.Body.Meal))
		})
	authed(s, api, huma.Operation{OperationID: "closeMonth", Method: http.MethodPost, Path: "/api/months/{month}/close", Tags: tags, Summary: "Freeze the month and generate the next report version"},
		func(ctx context.Context, u *app.User, in *monthIn) (*body[*app.Month], error) {
			return wrap(s.app.CloseMonth(ctx, u, in.Month))
		})
	authed(s, api, huma.Operation{OperationID: "reopenMonth", Method: http.MethodPost, Path: "/api/months/{month}/reopen", Tags: tags},
		func(ctx context.Context, u *app.User, in *monthBody[reasonBody]) (*body[*app.Month], error) {
			return wrap(s.app.ReopenMonth(ctx, u, in.Month, in.Body.Reason))
		})
	authed(s, api, huma.Operation{OperationID: "setMonthPaid", Method: http.MethodPost, Path: "/api/months/{month}/paid", Tags: tags},
		func(ctx context.Context, u *app.User, in *monthBody[paidBody]) (*body[*app.Month], error) {
			return wrap(s.app.SetPaid(ctx, u, in.Month, in.Body.PaidOn))
		})
	authed(s, api, huma.Operation{OperationID: "getReportPdf", Method: http.MethodGet, Path: "/api/reports/{id}/pdf", Tags: tags},
		func(ctx context.Context, u *app.User, in *idIn) (*fileOut, error) {
			b, name, err := s.app.ReportFile(ctx, u, in.ID)
			if err != nil {
				return nil, err
			}
			return &fileOut{ContentType: "application/pdf", ContentDisposition: attachment(name, false), CacheControl: "private, max-age=3600", Body: b}, nil
		})
}
