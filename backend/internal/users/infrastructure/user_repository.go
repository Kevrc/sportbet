package infrastructure

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/errs"
	"sportbet/internal/users/domain"
)

// UserRepository implementa application.UserRepository con pgx. Lee la tabla
// users que crea auth; las dos son de la Persona 1.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository { return &UserRepository{pool: pool} }

const columns = `id, email, display_name, role, status, created_at`

func scan(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "usuario no encontrado")
	}
	return u, err
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (domain.User, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+columns+` FROM users WHERE id = $1`, id))
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]domain.User, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+columns+` FROM users ORDER BY created_at DESC, id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.User, error) { return scan(row) })
	return users, total, err
}

// Update aplica solo los campos no nil de c.
func (r *UserRepository) Update(ctx context.Context, id string, c domain.Changes) (domain.User, error) {
	var role, status *string
	if c.Role != nil {
		role = (*string)(c.Role)
	}
	if c.Status != nil {
		status = (*string)(c.Status)
	}
	return scan(r.pool.QueryRow(ctx, `
		UPDATE users SET role = COALESCE($2, role), status = COALESCE($3, status)
		WHERE id = $1
		RETURNING `+columns, id, role, status))
}
