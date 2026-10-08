package presentation

import (
	"net/http"
	"slices"
	"strings"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/httpx"
)

type tokenParser interface {
	Parse(token string) (contracts.CurrentUser, error)
}

// Middleware implementa contracts.Authenticator con tokens Bearer.
type Middleware struct {
	tokens tokenParser
}

func NewMiddleware(tokens tokenParser) *Middleware { return &Middleware{tokens: tokens} }

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			httpx.WriteError(w, r, errs.New(errs.ErrUnauthorized, "UNAUTHORIZED", "token requerido"))
			return
		}
		u, err := m.tokens.Parse(raw)
		if err != nil {
			httpx.WriteError(w, r, errs.New(errs.ErrUnauthorized, "INVALID_TOKEN", "token inválido o vencido"))
			return
		}
		next.ServeHTTP(w, r.WithContext(contracts.WithCurrentUser(r.Context(), u)))
	})
}

func (m *Middleware) RequireRole(roles ...contracts.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := contracts.CurrentUserFrom(r.Context())
			if !ok || !slices.Contains(roles, u.Role) {
				httpx.WriteError(w, r, errs.New(errs.ErrForbidden, "FORBIDDEN", "permiso denegado"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
