package infrastructure

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/money"
)

type BetData struct {
	ID                string
	UserID            string
	Stake             money.Money
	Status            string
	SettlementVersion int
}

type BetSelectionData struct {
	Odd    money.Odds
	Result contracts.SelectionOutcome
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// UpdateEventSelections actualiza todas las selecciones pendientes de un evento de forma masiva.
func (r *Repository) UpdateEventSelections(ctx context.Context, eventID string, outcomes map[string]contracts.SelectionOutcome) error {
	for selID, outcome := range outcomes {
		_, err := r.pool.Exec(ctx, `
			UPDATE bet_selections
			SET result = $1
			WHERE event_id = $2 AND selection_id = $3 AND result = 'PENDING'
		`, outcome, eventID, selID)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetPendingBetsForEvent devuelve los IDs de apuestas que tienen alguna selección para este evento y están pendientes.
func (r *Repository) GetPendingBetsForEvent(ctx context.Context, eventID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT b.id
		FROM bets b
		JOIN bet_selections bs ON b.id = bs.bet_id
		WHERE bs.event_id = $1 AND b.status = 'PENDING'
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetBetWithSelections obtiene la cabecera de la apuesta y todas sus selecciones (no solo las del evento).
func (r *Repository) GetBetWithSelections(ctx context.Context, q db.Querier, betID string) (BetData, []BetSelectionData, error) {
	var bet BetData
	var stakeCents int64
	err := q.QueryRow(ctx, `
		SELECT id, user_id, stake_cents, status, settlement_version
		FROM bets
		WHERE id = $1
	`, betID).Scan(&bet.ID, &bet.UserID, &stakeCents, &bet.Status, &bet.SettlementVersion)
	if err != nil {
		return bet, nil, err
	}
	bet.Stake = money.FromCents(stakeCents)

	rows, err := q.Query(ctx, `
		SELECT odd_x100, result
		FROM bet_selections
		WHERE bet_id = $1
	`, betID)
	if err != nil {
		return bet, nil, err
	}
	defer rows.Close()

	var selections []BetSelectionData
	for rows.Next() {
		var odd int64
		var res string
		if err := rows.Scan(&odd, &res); err != nil {
			return bet, nil, err
		}
		selections = append(selections, BetSelectionData{
			Odd:    money.Odds(odd),
			Result: contracts.SelectionOutcome(res),
		})
	}
	return bet, selections, rows.Err()
}

// UpdateBetSettled actualiza la apuesta asegurando que estaba PENDING.
func (r *Repository) UpdateBetSettled(ctx context.Context, q db.Querier, betID string, status string, payout money.Money) (bool, error) {
	res, err := q.Exec(ctx, `
		UPDATE bets
		SET status = $1, payout_cents = $2, settled_at = now()
		WHERE id = $3 AND status = 'PENDING'
	`, status, payout.Cents(), betID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}
