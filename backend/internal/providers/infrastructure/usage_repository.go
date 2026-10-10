package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/providers/application"
	"sportbet/internal/shared/errs"
)

type UsageRepository struct {
	pool *pgxpool.Pool
}

func NewUsageRepository(pool *pgxpool.Pool) *UsageRepository {
	return &UsageRepository{pool: pool}
}

func (r *UsageRepository) Consume(ctx context.Context, provider string, day time.Time, limit int) error {
	if provider == "" || limit <= 0 {
		return errs.New(errs.ErrValidation, "INVALID_PROVIDER_BUDGET", "el proveedor y el límite de requests son obligatorios")
	}
	var requests int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO provider_usage (provider, day, requests)
		VALUES ($1, $2::date, 1)
		ON CONFLICT (provider, day)
		DO UPDATE SET requests = provider_usage.requests + 1
		WHERE provider_usage.requests < $3
		RETURNING requests`, provider, day.UTC().Format("2006-01-02"), limit).Scan(&requests)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.New(errs.ErrQuotaExceeded, "DAILY_PROVIDER_BUDGET", "se agotó el presupuesto diario del proveedor")
	}
	if err != nil {
		return fmt.Errorf("providers: registrar uso diario: %w", err)
	}
	return nil
}

func (r *UsageRepository) RequestsUsed(ctx context.Context, provider string, day time.Time) (int, error) {
	var requests int
	err := r.pool.QueryRow(ctx, `
		SELECT requests FROM provider_usage WHERE provider = $1 AND day = $2::date`,
		provider, day.UTC().Format("2006-01-02")).Scan(&requests)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("providers: consultar uso diario: %w", err)
	}
	return requests, nil
}

var _ application.UsageRepository = (*UsageRepository)(nil)
