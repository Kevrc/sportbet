package application

import (
	"context"
	"time"

	"sportbet/internal/auth/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
)

// UserRepository persiste usuarios. Create devuelve errs.ErrConflict con
// código EMAIL_TAKEN si el email ya existe; FindByEmail devuelve errs.ErrNotFound.
type UserRepository interface {
	Create(ctx context.Context, q db.Querier, u domain.User) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
}

// RefreshTokenRepository guarda solo el sha256 del refresh token.
type RefreshTokenRepository interface {
	Create(ctx context.Context, q db.Querier, userID string, tokenHash []byte, expiresAt time.Time) error
}

// AccessTokenIssuer firma el token de acceso (JWT).
type AccessTokenIssuer interface {
	Issue(u contracts.CurrentUser) (token string, expiresAt time.Time, err error)
}

// TxRunner ejecuta fn dentro de una transacción.
type TxRunner func(ctx context.Context, fn func(q db.Querier) error) error
