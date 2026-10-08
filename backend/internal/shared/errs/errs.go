// Package errs define los errores comunes de todos los módulos.
// httpx.WriteError los traduce a códigos HTTP.
package errs

import "errors"

// Sentinels. Se comparan con errors.Is.
var (
	ErrNotFound            = errors.New("not found")
	ErrValidation          = errors.New("validation")
	ErrConflict            = errors.New("conflict")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrMarketClosed        = errors.New("market closed")
	ErrOddsChanged         = errors.New("odds changed")
	ErrAlreadyProcessed    = errors.New("already processed")
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrQuotaExceeded       = errors.New("quota exceeded")
)

// Error lleva un sentinel (Kind), un código estable para el cliente
// (por ejemplo "INSUFFICIENT_FUNDS") y opcionalmente la causa original.
type Error struct {
	Kind    error
	Code    string
	Message string
	Fields  map[string]string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap permite errors.Is tanto con el sentinel como con la causa.
func (e *Error) Unwrap() []error {
	out := []error{e.Kind}
	if e.Cause != nil {
		out = append(out, e.Cause)
	}
	return out
}

func New(kind error, code, msg string) *Error {
	return &Error{Kind: kind, Code: code, Message: msg}
}

func Wrap(kind, cause error, code, msg string) *Error {
	return &Error{Kind: kind, Code: code, Message: msg, Cause: cause}
}

// Validation crea un error 400 con el detalle de cada campo.
func Validation(fields map[string]string) *Error {
	return &Error{Kind: ErrValidation, Code: "VALIDATION_ERROR", Message: "datos inválidos", Fields: fields}
}
