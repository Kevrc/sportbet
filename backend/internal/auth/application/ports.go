package application

import (
	"context"
	"time"

	"sportbet/internal/auth/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
)

// UserRepository persiste usuarios. Create devuelve errs.ErrConflict con
// código EMAIL_TAKEN si el email ya existe; FindByEmail y FindByID devuelven
// errs.ErrNotFound.
type UserRepository interface {
	Create(ctx context.Context, q db.Querier, u domain.User) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindByID(ctx context.Context, q db.Querier, id string) (domain.User, error)
}

// RefreshTokenRepository guarda solo el sha256 del refresh token.
type RefreshTokenRepository interface {
	Create(ctx context.Context, q db.Querier, userID string, tokenHash []byte, expiresAt time.Time) error
	// Consume revoca el token si está vigente y devuelve su usuario, en una
	// sola sentencia: dos refresh simultáneos con el mismo token no pueden
	// ganar los dos. Devuelve errs.ErrUnauthorized si no existe, venció o ya
	// estaba revocado.
	Consume(ctx context.Context, q db.Querier, tokenHash []byte, now time.Time) (userID string, err error)
	// Revoke revoca el token. Es idempotente: revocar uno inexistente o ya
	// revocado no es un error.
	Revoke(ctx context.Context, q db.Querier, tokenHash []byte, now time.Time) error
}

// AccessTokenIssuer firma el token de acceso (JWT).
type AccessTokenIssuer interface {
	Issue(u contracts.CurrentUser) (token string, expiresAt time.Time, err error)
}

// TxRunner ejecuta fn dentro de una transacción.
type TxRunner func(ctx context.Context, fn func(q db.Querier) error) error
