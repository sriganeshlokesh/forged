// Package error_code defines structured application error codes.
// Error code ranges:
//
//	10000+ — API / request errors
//	20000+ — authentication / authorization errors
//	30000+ — internal / infrastructure errors
//	40000+ — business-logic errors
package error_code

import "fmt"

// Error is a structured application error that carries an internal code,
// a human-readable message, and an HTTP status code for response mapping.
type Error struct {
	Code int
	Msg  string
	HTTP int
}

// New constructs a new Error.
func New(code int, msg string, httpStatus int) *Error {
	return &Error{Code: code, Msg: msg, HTTP: httpStatus}
}

func (e *Error) Error() string {
	return fmt.Sprintf("error %d: %s", e.Code, e.Msg)
}

// Predefined errors.
var (
	ErrInvalidParams = New(10001, "invalid params", 400)
	ErrInternal      = New(30001, "internal error", 500)
)
