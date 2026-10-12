package contracts

import (
	"context"
	"time"
)

// Implementa y usa: Persona 2 (providers, events). Implementaciones:
// ReplayProvider (propio) y API-Football. Ambas devuelven
// errs.ErrProviderUnavailable o errs.ErrQuotaExceeded cuando fallan.

type ProviderTeam struct {
	ExternalID string
	Name       string
	// LogoURL es la URL https del escudo, o vacío si el proveedor no la da.
	LogoURL string
}

type ProviderLeague struct {
	ExternalID string
	Name       string
	Country    string
}

type ProviderFixture struct {
	ExternalID string
	League     ProviderLeague
	Home       ProviderTeam
	Away       ProviderTeam
	StartAt    time.Time
	Status     MatchStatus
	// Marcador de tiempo reglamentario; nil mientras no haya empezado.
	HomeScore *int
	AwayScore *int
}

type LiveScore struct {
	ExternalID string
	Status     MatchStatus
	HomeScore  int
	AwayScore  int
	Minute     int
	ObservedAt time.Time
}

// FixtureProvider entrega el calendario y los resultados finales.
type FixtureProvider interface {
	Name() string
	FixturesByDate(ctx context.Context, date time.Time, leagueIDs []string) ([]ProviderFixture, error)
	FixtureResult(ctx context.Context, externalID string) (ProviderFixture, error)
}

// LiveScoreProvider entrega el marcador en vivo de todos los partidos en juego
// en una sola llamada (fixtures?live=all en API-Football).
type LiveScoreProvider interface {
	Name() string
	LiveScores(ctx context.Context) ([]LiveScore, error)
}
