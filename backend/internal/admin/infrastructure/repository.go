package infrastructure

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/admin/domain"
)

// Repository implementa application.Repository con pgx sobre audit_log.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Record(ctx context.Context, e domain.AuditEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, entity, entity_id, data)
		VALUES ($1::uuid, $2, $3, $4, $5)`, e.ActorID, e.Action, e.Entity, e.EntityID, e.Data)
	if err != nil {
		return fmt.Errorf("admin: registrar auditoría: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]domain.AuditEntry, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("admin: contar auditoría: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.actor_id::text, u.display_name, a.action, a.entity, a.entity_id, a.data, a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_id
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin: listar auditoría: %w", err)
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.AuditEntry, error) {
		var e domain.AuditEntry
		err := row.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Action, &e.Entity, &e.EntityID, &e.Data, &e.CreatedAt)
		return e, err
	})
	return entries, total, err
}

func (r *Repository) Stats(ctx context.Context) (domain.Stats, error) {
	var s domain.Stats
	err := r.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM users),
		       count(*) FILTER (WHERE action = $1),
		       count(*) FILTER (WHERE action = $2),
		       COALESCE(sum((data->>'stakeCents')::bigint) FILTER (WHERE action = $1), 0),
		       COALESCE(sum((data->>'payoutCents')::bigint) FILTER (WHERE action = $2), 0)
		FROM audit_log`, domain.ActionBetPlaced, domain.ActionBetSettled).
		Scan(&s.Users, &s.BetsPlaced, &s.BetsSettled, &s.StakeCents, &s.PayoutCents)
	if err != nil {
		return s, fmt.Errorf("admin: calcular estadísticas: %w", err)
	}
	return s, nil
}
