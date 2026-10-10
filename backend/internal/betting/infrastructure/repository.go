package infrastructure

import (
	"context"
	"fmt"

	"sportbet/internal/betting/application"
	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/db"
)

type repository struct{}

func NewRepository() application.Repository {
	return &repository{}
}

func (r *repository) InsertBet(ctx context.Context, q db.Querier, bet *domain.Bet, selections []domain.BetSelection) (string, error) {
	var betID string
	err := q.QueryRow(ctx, `
		INSERT INTO bets (user_id, idempotency_key, stake_cents, total_odd_x100, potential_payout_cents, status, payout_cents, settlement_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, bet.UserID, bet.IdempotencyKey, bet.Stake.Cents(), bet.TotalOdd, bet.PotentialPayout.Cents(), bet.Status, bet.Payout.Cents(), bet.SettlementVersion).Scan(&betID)

	if err != nil {
		if db.IsUniqueViolation(err, "bets_user_idempotency_key") {
			return "", fmt.Errorf("idempotency violation: %w", err)
		}
		return "", fmt.Errorf("insert bet: %w", err)
	}

	for _, sel := range selections {
		_, err := q.Exec(ctx, `
			INSERT INTO bet_selections (bet_id, selection_id, event_id, odd_x100, result)
			VALUES ($1, $2, $3, $4, $5)
		`, betID, sel.SelectionID, sel.EventID, sel.Odd, sel.Result)
		if err != nil {
			return "", fmt.Errorf("insert selection %s: %w", sel.SelectionID, err)
		}
	}

	return betID, nil
}
