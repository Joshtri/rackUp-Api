// Package httpx holds the HTTP helpers shared by every feature:
// the error type, JSON in/out, and path params.
package httpx

import (
	"errors"
	"log/slog"
	"net/http"
)

// Error is every error a handler can return.
// It is sent as {"error": {"code", "message"}}.
type Error struct {
	Status  int
	Code    string
	Message string
	// Cause is logged for server errors, never sent to the client.
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

var (
	ErrUnauthorized       = &Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "login required"}
	ErrInvalidCredentials = &Error{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "wrong username or password"}
	ErrAccountExpired     = &Error{Status: http.StatusUnauthorized, Code: "account_expired", Message: "account has expired"}
	ErrNotFound           = &Error{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}
)

// Validation is a 422 for input that parsed but breaks a rule.
func Validation(msg string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_error", Message: msg}
}

// BadRequest is a malformed request: bad JSON, bad path param, wrong content type.
func BadRequest(status int, msg string) *Error {
	return &Error{Status: status, Code: "bad_request", Message: msg}
}

func Conflict(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: "conflict", Message: msg}
}

// Internal is a 500. The client only sees "something went wrong".
func Internal(cause error) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: "internal_error", Message: "something went wrong", Cause: cause}
}

// HandlerFunc is an http.HandlerFunc that can return an error.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		WriteError(w, r, err)
	}
}

// WriteError sends err in the JSON error format. Errors that aren't *Error become 500s.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = Internal(err)
	}
	if e.Status >= 500 {
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	}
	WriteJSON(w, e.Status, map[string]any{
		"error": map[string]string{"code": e.Code, "message": e.Message},
	})
}
