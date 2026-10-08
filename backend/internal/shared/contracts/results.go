package contracts

import (
	"context"
	"time"
)

// Implementa: Persona 2 (markets). Lo usa: Persona 3 (settlement).

// EventResult usa el marcador de tiempo reglamentario (score.fulltime de
// API-Football: 90 min + añadido, sin prórroga ni penales).
type EventResult struct {
	EventID    string
	Status     MatchStatus
	HomeScore  int
	AwayScore  int
	FinishedAt time.Time
}

type ResultReader interface {
	// GetResult devuelve errs.ErrNotFound si el evento no existe y
	// errs.ErrConflict con código EVENT_NOT_FINISHED si no está en estado terminal.
	GetResult(ctx context.Context, eventID string) (EventResult, error)
	// ResolveSelections devuelve WON, LOST o VOID para cada selección del
	// evento. Si el evento terminó en POSTPONED, CANCELLED, ABANDONED o
	// AWARDED, todas son VOID. Saber qué gana es lógica de markets.
	ResolveSelections(ctx context.Context, eventID string) (map[string]SelectionOutcome, error)
	// ListFinishedSince alimenta el barrido de respaldo de settlement.
	ListFinishedSince(ctx context.Context, since time.Time) ([]EventResult, error)
}
