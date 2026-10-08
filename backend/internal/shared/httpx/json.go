// Package httpx tiene utilidades HTTP comunes: JSON, errores y paginación.
package httpx

import (
	"encoding/json"
	"net/http"
	"strconv"

	"sportbet/internal/shared/errs"
)

const maxBody = 1 << 20

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// DecodeJSON lee el cuerpo rechazando campos desconocidos y cuerpos de más de 1 MB.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errs.Wrap(errs.ErrValidation, err, "INVALID_JSON", "cuerpo JSON inválido")
	}
	return nil
}

type Page struct {
	Page     int
	PageSize int
}

func (p Page) Offset() int { return (p.Page - 1) * p.PageSize }

// ParsePage lee ?page y ?pageSize (por defecto 1 y 20, máximo 100).
func ParsePage(r *http.Request) Page {
	p := Page{Page: 1, PageSize: 20}
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		p.Page = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("pageSize")); err == nil && v > 0 {
		p.PageSize = min(v, 100)
	}
	return p
}

type Paged[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}
