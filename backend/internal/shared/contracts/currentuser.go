package contracts

import (
	"context"
	"net/http"
)

// Implementa: Persona 1 (auth). Lo usan: todos.

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type CurrentUser struct {
	ID   string
	Role Role
}

func (u CurrentUser) IsAdmin() bool { return u.Role == RoleAdmin }

// Authenticator es el middleware que valida el token de acceso y deja el
// CurrentUser en el contexto del request.
type Authenticator interface {
	// RequireAuth responde 401 si no hay token válido.
	RequireAuth(next http.Handler) http.Handler
	// RequireRole responde 403 si el usuario no tiene alguno de los roles.
	// Se usa siempre después de RequireAuth.
	RequireRole(roles ...Role) func(http.Handler) http.Handler
}

type currentUserKey struct{}

func WithCurrentUser(ctx context.Context, u CurrentUser) context.Context {
	return context.WithValue(ctx, currentUserKey{}, u)
}

func CurrentUserFrom(ctx context.Context) (CurrentUser, bool) {
	u, ok := ctx.Value(currentUserKey{}).(CurrentUser)
	return u, ok
}

// MustCurrentUser solo se usa en handlers protegidos por RequireAuth.
func MustCurrentUser(ctx context.Context) CurrentUser {
	u, ok := CurrentUserFrom(ctx)
	if !ok {
		panic("contracts: MustCurrentUser sin RequireAuth")
	}
	return u
}
