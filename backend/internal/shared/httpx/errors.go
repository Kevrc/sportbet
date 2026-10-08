package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"sportbet/internal/shared/errs"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

var statusByKind = []struct {
	kind   error
	status int
}{
	{errs.ErrNotFound, http.StatusNotFound},
	{errs.ErrValidation, http.StatusBadRequest},
	{errs.ErrUnauthorized, http.StatusUnauthorized},
	{errs.ErrForbidden, http.StatusForbidden},
	{errs.ErrConflict, http.StatusConflict},
	{errs.ErrOddsChanged, http.StatusConflict},
	{errs.ErrAlreadyProcessed, http.StatusConflict},
	{errs.ErrInsufficientFunds, http.StatusUnprocessableEntity},
	{errs.ErrMarketClosed, http.StatusUnprocessableEntity},
	{errs.ErrProviderUnavailable, http.StatusServiceUnavailable},
	{errs.ErrQuotaExceeded, http.StatusServiceUnavailable},
}

// StatusOf devuelve el código HTTP de un error (500 si no es conocido).
func StatusOf(err error) int {
	for _, s := range statusByKind {
		if errors.Is(err, s.kind) {
			return s.status
		}
	}
	return http.StatusInternalServerError
}

// WriteError responde {"error":{"code","message","fields?"}}.
// Los errores desconocidos se registran y se responden con un mensaje genérico.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	status := StatusOf(err)
	var e *errs.Error
	if status == http.StatusInternalServerError || !errors.As(err, &e) {
		if status == http.StatusInternalServerError {
			slog.ErrorContext(r.Context(), "error interno", "err", err, "path", r.URL.Path)
			WriteJSON(w, status, errorBody{errorDetail{Code: "INTERNAL", Message: "error interno"}})
			return
		}
		WriteJSON(w, status, errorBody{errorDetail{Code: http.StatusText(status), Message: err.Error()}})
		return
	}
	WriteJSON(w, status, errorBody{errorDetail{Code: e.Code, Message: e.Message, Fields: e.Fields}})
}
