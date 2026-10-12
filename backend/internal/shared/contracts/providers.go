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
	// Season es la temporada en curso según el proveedor (0 si no la da).
	Season int
	// LogoURL y FlagURL son URLs https, o vacío si el proveedor no las da.
	LogoURL string
	FlagURL string
}

type ProviderFixture struct {
	ExternalID string
	League     ProviderLeague
	Home       ProviderTeam
	Away       ProviderTeam
	StartAt    time.Time
	Status     MatchStatus
	// Round es la jornada tal como la nombra el proveedor ("Regular Season - 8").
	Round     string
	VenueName string
	VenueCity string
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
	// Events es la lista completa de sucesos del partido hasta ahora, en
	// orden. Vacía significa "sin datos" y no borra los ya guardados.
	Events []MatchEvent
}

// Tipos de suceso y lado del equipo en MatchEvent.
const (
	MatchEventGoal  = "GOAL"
	MatchEventCard  = "CARD"
	MatchEventSubst = "SUBST"
	MatchEventVAR   = "VAR"

	SideHome = "HOME"
	SideAway = "AWAY"
)

// MatchEvent es un suceso del partido: gol, tarjeta, cambio o revisión VAR.
type MatchEvent struct {
	Minute int
	Extra  int    // minutos de descuento: 90+3 es Minute 90, Extra 3
	Side   string // SideHome | SideAway
	Type   string // MatchEventGoal | MatchEventCard | MatchEventSubst | MatchEventVAR
	Detail string // texto del proveedor: "Normal Goal", "Penalty", "Yellow Card"…
	Player string
	Assist string // asistencia; en un cambio, el otro jugador
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

// TeamStanding es la fila de un equipo en la tabla de posiciones.
type TeamStanding struct {
	TeamExternalID string
	Played         int
	GoalsFor       int
	GoalsAgainst   int
}

// StandingsProvider es opcional: solo lo implementan los proveedores que
// entregan tablas de posiciones (API-Football). Se usa para calcular la
// fuerza de los equipos que alimenta el modelo de cuotas.
type StandingsProvider interface {
	Standings(ctx context.Context, leagueExternalID string, season int) ([]TeamStanding, error)
}
