package presentation

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type betResponse struct {
	ID                   string                 `json:"id"`
	StakeCents           int64                  `json:"stakeCents"`
	TotalOddX100         int64                  `json:"totalOddX100"`
	PotentialPayoutCents int64                  `json:"potentialPayoutCents"`
	Status               string                 `json:"status"`
	PayoutCents          int64                  `json:"payoutCents"`
	PlacedAt             interface{}            `json:"placedAt"`
	Selections           []betSelectionResponse `json:"selections"`
}

type betSelectionResponse struct {
	SelectionID   string `json:"selectionId"`
	EventID       string `json:"eventId"`
	EventName     string `json:"eventName"`
	MarketCode    string `json:"marketCode"`
	SelectionCode string `json:"selectionCode"`
	Label         string `json:"label"`
	OddX100       int64  `json:"oddX100"`
	Result        string `json:"result"`
}

func fetchBetComplete(ctx context.Context, pool *pgxpool.Pool, betID string, userID string) (*betResponse, error) {
	var bet betResponse
	err := pool.QueryRow(ctx, `
		SELECT 
			b.id, b.stake_cents, b.total_odd_x100, b.potential_payout_cents, b.status, b.payout_cents, b.placed_at,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'selectionId', bs.selection_id,
					'eventId', bs.event_id,
					'eventName', e.name,
					'marketCode', m.code,
					'selectionCode', s.code,
					'label', s.name,
					'oddX100', bs.odd_x100,
					'result', bs.result
				)) 
				FROM bet_selections bs 
				JOIN selections s ON bs.selection_id = s.id
				JOIN markets m ON s.market_id = m.id
				JOIN events e ON bs.event_id = e.id
				WHERE bs.bet_id = b.id), '[]'::json
			) as selections
		FROM bets b
		WHERE b.id = $1 AND b.user_id = $2
	`, betID, userID).Scan(&bet.ID, &bet.StakeCents, &bet.TotalOddX100, &bet.PotentialPayoutCents, &bet.Status, &bet.PayoutCents, &bet.PlacedAt, &bet.Selections)
	if err != nil {
		return nil, err
	}
	return &bet, nil
}

func fetchBetByIdempotencyKey(ctx context.Context, pool *pgxpool.Pool, idemKey string, userID string) (*betResponse, error) {
	var betID string
	err := pool.QueryRow(ctx, "SELECT id FROM bets WHERE idempotency_key = $1 AND user_id = $2", idemKey, userID).Scan(&betID)
	if err != nil {
		return nil, err
	}
	return fetchBetComplete(ctx, pool, betID, userID)
}
