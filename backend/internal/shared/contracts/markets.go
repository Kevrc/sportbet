package contracts

import (
	"context"
	"time"

	"sportbet/internal/shared/money"
)

// Implementa: Persona 2 (markets). Lo usa: Persona 3 (betting).

// MarketCloseLead es cuánto antes de start_at deja de aceptarse una apuesta.
const MarketCloseLead = 60 * time.Second

type SelectionInfo struct {
	SelectionID   string
	SelectionCode string // HOME, DRAW, AWAY, OVER, UNDER, YES, NO
	MarketID      string
	MarketCode    string // MATCH_RESULT, TOTAL_GOALS_2_5, BOTH_TEAMS_TO_SCORE
	EventID       string
	EventName     string // "Local vs Visitante"
	Label         string
	Odd           money.Odds
	MarketStatus  MarketStatus
	EventStatus   MatchStatus
	EventStartAt  time.Time
}

// IsOpen no confía solo en MatchStarted: también cierra por reloj, porque el
// bus puede perder eventos y no hay polling de partidos no destacados.
func (s SelectionInfo) IsOpen(now time.Time) bool {
	return s.MarketStatus == MarketOpen &&
		s.EventStatus == MatchScheduled &&
		now.Before(s.EventStartAt.Add(-MarketCloseLead))
}

type MarketReader interface {
	// GetSelections devuelve las selecciones pedidas. Las que no existen no
	// aparecen en el mapa.
	GetSelections(ctx context.Context, ids []string) (map[string]SelectionInfo, error)
}
