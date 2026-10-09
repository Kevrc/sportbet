package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/markets/application"
	"sportbet/internal/markets/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/money"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListEventMarkets(ctx context.Context, eventID string) ([]application.Market, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id::text, m.event_id::text, m.code, m.status,
		       s.id::text, s.code, s.label, s.odd_x100, s.result
		FROM markets m
		LEFT JOIN selections s ON s.market_id = m.id
		WHERE m.event_id = $1::uuid
		ORDER BY m.code, s.id`, eventID)
	if err != nil {
		return nil, fmt.Errorf("markets: listar mercados: %w", err)
	}
	defer rows.Close()

	markets := make([]application.Market, 0)
	index := make(map[string]int)
	for rows.Next() {
		var marketID, eventKey, code, status string
		var selectionID, selectionCode, label, result *string
		var oddX100 *int32
		if err := rows.Scan(&marketID, &eventKey, &code, &status, &selectionID, &selectionCode, &label, &oddX100, &result); err != nil {
			return nil, fmt.Errorf("markets: leer mercado: %w", err)
		}
		position, exists := index[marketID]
		if !exists {
			position = len(markets)
			index[marketID] = position
			markets = append(markets, application.Market{
				ID: marketID, EventID: eventKey, Code: code, Status: contracts.MarketStatus(status),
				Selections: make([]application.Selection, 0),
			})
		}
		if selectionID != nil {
			markets[position].Selections = append(markets[position].Selections, application.Selection{
				ID: *selectionID, Code: value(selectionCode), Label: value(label),
				OddX100: int64(value(oddX100)), Result: contracts.SelectionOutcome(value(result)),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("markets: recorrer mercados: %w", err)
	}
	return markets, nil
}

func (r *Repository) EnsureMarkets(ctx context.Context, eventID string, margin int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("markets: iniciar transacción de mercados: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var homeStrength, awayStrength float64
	err = tx.QueryRow(ctx, `
		SELECT home.strength, away.strength
		FROM events e
		JOIN teams home ON home.id = e.home_team_id
		JOIN teams away ON away.id = e.away_team_id
		WHERE e.id = $1::uuid AND e.status = 'SCHEDULED'`, eventID).Scan(&homeStrength, &awayStrength)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.New(errs.ErrNotFound, "EVENT_NOT_FOUND", "evento programado no encontrado")
	}
	if err != nil {
		return fmt.Errorf("markets: leer fortalezas de equipos: %w", err)
	}
	odds, err := domain.CalculateOdds(homeStrength, awayStrength, margin)
	if err != nil {
		return fmt.Errorf("markets: calcular cuotas: %w", err)
	}
	sets := []struct {
		code       string
		selections []struct {
			code  string
			label string
			odd   money.Odds
		}
	}{
		{contracts.MarketMatchResult, []struct {
			code  string
			label string
			odd   money.Odds
		}{{"HOME", "Gana local", odds.MatchResult[0]}, {"DRAW", "Empate", odds.MatchResult[1]}, {"AWAY", "Gana visitante", odds.MatchResult[2]}}},
		{contracts.MarketTotalGoals, []struct {
			code  string
			label string
			odd   money.Odds
		}{{"OVER", "Más de 2.5 goles", odds.TotalGoals[0]}, {"UNDER", "Menos de 2.5 goles", odds.TotalGoals[1]}}},
		{contracts.MarketBTTS, []struct {
			code  string
			label string
			odd   money.Odds
		}{{"YES", "Ambos equipos marcan", odds.BothScore[0]}, {"NO", "No marcan ambos equipos", odds.BothScore[1]}}},
	}

	for _, market := range sets {
		var marketID string
		err := tx.QueryRow(ctx, `
			INSERT INTO markets (event_id, code, status)
			VALUES ($1::uuid, $2, 'OPEN')
			ON CONFLICT (event_id, code) DO UPDATE SET code = EXCLUDED.code
			RETURNING id::text`, eventID, market.code).Scan(&marketID)
		if err != nil {
			return fmt.Errorf("markets: crear mercado %s: %w", market.code, err)
		}
		for _, selection := range market.selections {
			_, err := tx.Exec(ctx, `
				INSERT INTO selections (market_id, code, label, odd_x100)
				VALUES ($1::uuid, $2, $3, $4)
				ON CONFLICT (market_id, code) DO NOTHING`,
				marketID, selection.code, selection.label, int64(selection.odd))
			if err != nil {
				return fmt.Errorf("markets: crear selección %s: %w", selection.code, err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("markets: confirmar mercados: %w", err)
	}
	return nil
}

func (r *Repository) CloseMarketsForEvent(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE markets SET status = 'CLOSED'
		WHERE event_id = $1::uuid AND status = 'OPEN'`, eventID)
	if err != nil {
		return fmt.Errorf("markets: cerrar mercados: %w", err)
	}
	return nil
}

func (r *Repository) GetSelections(ctx context.Context, ids []string) (map[string]contracts.SelectionInfo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id::text, s.code, s.market_id::text, m.code, m.event_id::text,
		       s.label, s.odd_x100, m.status, e.status, e.start_at
		FROM selections s
		JOIN markets m ON m.id = s.market_id
		JOIN events e ON e.id = m.event_id
		WHERE s.id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("markets: consultar selecciones: %w", err)
	}
	defer rows.Close()
	result := make(map[string]contracts.SelectionInfo, len(ids))
	for rows.Next() {
		var item contracts.SelectionInfo
		var marketStatus, eventStatus string
		var oddX100 int32
		if err := rows.Scan(
			&item.SelectionID, &item.SelectionCode, &item.MarketID, &item.MarketCode,
			&item.EventID, &item.Label, &oddX100, &marketStatus, &eventStatus, &item.EventStartAt,
		); err != nil {
			return nil, fmt.Errorf("markets: leer selección: %w", err)
		}
		item.Odd = money.Odds(oddX100)
		item.MarketStatus = contracts.MarketStatus(marketStatus)
		item.EventStatus = contracts.MatchStatus(eventStatus)
		result[item.SelectionID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("markets: recorrer selecciones: %w", err)
	}
	return result, nil
}

func (r *Repository) GetEventResult(ctx context.Context, eventID string) (contracts.EventResult, error) {
	var result contracts.EventResult
	var status string
	var home, away *int32
	var finishedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, status, home_score, away_score, finished_at
		FROM events WHERE id = $1::uuid`, eventID).
		Scan(&result.EventID, &status, &home, &away, &finishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.EventResult{}, errs.New(errs.ErrNotFound, "EVENT_NOT_FOUND", "evento no encontrado")
	}
	if err != nil {
		return contracts.EventResult{}, fmt.Errorf("markets: consultar resultado: %w", err)
	}
	result.Status = contracts.MatchStatus(status)
	if !result.Status.IsTerminal() {
		return contracts.EventResult{}, errs.New(errs.ErrConflict, "EVENT_NOT_FINISHED", "el evento todavía no terminó")
	}
	if result.Status == contracts.MatchFinished && (home == nil || away == nil) {
		return contracts.EventResult{}, errs.New(errs.ErrConflict, "EVENT_RESULT_MISSING", "el resultado final no tiene marcador reglamentario")
	}
	if home != nil {
		result.HomeScore = int(*home)
	}
	if away != nil {
		result.AwayScore = int(*away)
	}
	if finishedAt != nil {
		result.FinishedAt = *finishedAt
	}
	return result, nil
}

func (r *Repository) ListFinishedSince(ctx context.Context, since time.Time) ([]contracts.EventResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, status, COALESCE(home_score, 0), COALESCE(away_score, 0), COALESCE(finished_at, start_at)
		FROM events
		WHERE status IN ('FINISHED', 'POSTPONED', 'CANCELLED', 'ABANDONED', 'AWARDED')
		  AND COALESCE(finished_at, start_at) >= $1
		ORDER BY COALESCE(finished_at, start_at)`, since)
	if err != nil {
		return nil, fmt.Errorf("markets: listar eventos terminales: %w", err)
	}
	defer rows.Close()
	results := make([]contracts.EventResult, 0)
	for rows.Next() {
		var result contracts.EventResult
		var status string
		if err := rows.Scan(&result.EventID, &status, &result.HomeScore, &result.AwayScore, &result.FinishedAt); err != nil {
			return nil, fmt.Errorf("markets: leer evento terminal: %w", err)
		}
		result.Status = contracts.MatchStatus(status)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("markets: recorrer eventos terminales: %w", err)
	}
	return results, nil
}

func (r *Repository) ListEventSelections(ctx context.Context, eventID string) ([]application.SelectionCode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id::text, s.code
		FROM selections s JOIN markets m ON m.id = s.market_id
		WHERE m.event_id = $1::uuid
		ORDER BY s.id`, eventID)
	if err != nil {
		return nil, fmt.Errorf("markets: listar selecciones del evento: %w", err)
	}
	defer rows.Close()
	selections := make([]application.SelectionCode, 0)
	for rows.Next() {
		var selection application.SelectionCode
		if err := rows.Scan(&selection.ID, &selection.Code); err != nil {
			return nil, fmt.Errorf("markets: leer selección del evento: %w", err)
		}
		selections = append(selections, selection)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("markets: recorrer selecciones del evento: %w", err)
	}
	return selections, nil
}

func value[T any](item *T) T {
	if item == nil {
		var zero T
		return zero
	}
	return *item
}

var _ application.Repository = (*Repository)(nil)
