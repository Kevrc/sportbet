package infrastructure

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
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

func (RefreshTokenRepository) Consume(ctx context.Context, q db.Querier, tokenHash []byte, now time.Time) (string, error) {
	var userID string
	err := q.QueryRow(ctx, `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
		RETURNING user_id`,
		tokenHash, now,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errs.New(errs.ErrUnauthorized, "INVALID_REFRESH_TOKEN", "sesión vencida o cerrada")
	}
	return userID, err
}

func (RefreshTokenRepository) Revoke(ctx context.Context, q db.Querier, tokenHash []byte, now time.Time) error {
	_, err := q.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash, now)
	return err
}
