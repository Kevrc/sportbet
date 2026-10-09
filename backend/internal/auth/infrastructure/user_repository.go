package infrastructure

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/auth/domain"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
)

// UserRepository implementa application.UserRepository con pgx.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository { return &UserRepository{pool: pool} }

const userColumns = `id, email, password_hash, display_name, role, status, created_at`

func (r *UserRepository) Create(ctx context.Context, q db.Querier, u domain.User) (domain.User, error) {
	err := q.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		u.Email, u.PasswordHash, u.DisplayName, u.Role, u.Status,
	).Scan(&u.ID, &u.CreatedAt)
	if db.IsUniqueViolation(err, "users_email_key") {
		return u, errs.Wrap(errs.ErrConflict, err, "EMAIL_TAKEN", "el email ya está registrado")
	}
	return u, err
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "usuario no encontrado")
	}
	return u, err
}

func (r *UserRepository) FindByID(ctx context.Context, q db.Querier, id string) (domain.User, error) {
	var u domain.User
	err := q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "usuario no encontrado")
	}
	return u, err
}
