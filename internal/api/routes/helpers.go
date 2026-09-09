package routes

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/shared"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ErrorResponse struct {
	Message string `json:"message" description:"Human readable description of the failure"`
	Code    int    `json:"code" description:"HTTP status code"`
}

func (rt *Routes) WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		rt.Logger.Error("failed encoding response body", slog.Any("error", err))
	}
}

// WriteAppError renders a *shared.AppError and records it on the span.
func (rt *Routes) WriteAppError(w http.ResponseWriter, span trace.Span, appErr *shared.AppError) {
	code := appErr.Code
	if code == 0 {
		code = http.StatusInternalServerError
	}
	msg := appErr.Message
	if msg == "" {
		msg = http.StatusText(code)
	}
	if span != nil {
		span.SetStatus(codes.Error, msg)
		if appErr.Err != nil {
			span.RecordError(appErr.Err)
		}
	}
	rt.Logger.Error("request failed", slog.Int("code", code), slog.String("message", msg))
	rt.WriteJSON(w, code, ErrorResponse{Message: msg, Code: code})
}

// WriteError renders a plain error that never reached the service layer, such
// as a malformed body or a failed validation.
func (rt *Routes) WriteError(w http.ResponseWriter, span trace.Span, code int, msg string, err error) {
	rt.WriteAppError(w, span, &shared.AppError{Code: code, Message: msg, Err: err})
}

// DecodeAndValidate reads the JSON body into dst and runs struct validation.
// It reports whether the handler should continue.
func (rt *Routes) DecodeAndValidate(w http.ResponseWriter, r *http.Request, span trace.Span, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		rt.WriteError(w, span, http.StatusBadRequest, "Invalid request body", err)
		return false
	}
	if err := rt.Validator.Struct(dst); err != nil {
		rt.WriteError(w, span, http.StatusUnprocessableEntity, err.Error(), err)
		return false
	}
	return true
}
