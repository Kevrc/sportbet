// Package db conecta con PostgreSQL, aplica migraciones y ofrece transacciones.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Querier lo cumplen *pgxpool.Pool y pgx.Tx. Los contratos entre módulos
// lo reciben para que una operación participe en la transacción de otro.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// Migrate aplica las migraciones de fsys con goose. Permite migraciones fuera
// de orden porque cada persona usa su propio prefijo (1xxx_, 2xxx_, 3xxx_).
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithAllowOutofOrder(true))
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}

// WithTx ejecuta fn en una transacción; hace rollback si fn devuelve error.
// Los eventos del bus se publican después de que WithTx devuelva nil.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q Querier) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsUniqueViolation indica si err es una violación de la restricción única
// indicada. Con constraint vacío acepta cualquier restricción única.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}
