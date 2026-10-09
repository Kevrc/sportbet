package infrastructure

import (
	"context"
	"time"

	"sportbet/internal/shared/db"
)

// RefreshTokenRepository implementa application.RefreshTokenRepository con pgx.
type RefreshTokenRepository struct{}

func NewRefreshTokenRepository() *RefreshTokenRepository { return &RefreshTokenRepository{} }

func (RefreshTokenRepository) Create(ctx context.Context, q db.Querier, userID string, tokenHash []byte, expiresAt time.Time) error {
	_, err := q.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}
