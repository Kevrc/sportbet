package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type replayFile struct {
	Fixture replayFixture `json:"fixture"`
	Frames  []replayFrame `json:"frames"`
}

type replayFixture struct {
	ExternalID string                   `json:"externalId"`
	League     contracts.ProviderLeague `json:"league"`
	Home       contracts.ProviderTeam   `json:"home"`
	Away       contracts.ProviderTeam   `json:"away"`
	StartAt    *time.Time               `json:"startAt"`
}

type replayFrame struct {
	OffsetSeconds int64  `json:"offsetSeconds"`
	Status        string `json:"status"`
	Home          int    `json:"home"`
	Away          int    `json:"away"`
	Minute        int    `json:"minute"`
	// Events son los sucesos que ocurren en este cuadro (no acumulados).
	Events []replayEvent `json:"events,omitempty"`
}

type replayEvent struct {
	Minute int    `json:"minute"`
	Extra  int    `json:"extra,omitempty"`
	Side   string `json:"side"`
	Type   string `json:"type"`
	Detail string `json:"detail"`
	Player string `json:"player"`
	Assist string `json:"assist,omitempty"`
}

// eventsUntil acumula los sucesos de todos los cuadros hasta offset.
func (p *ReplayProvider) eventsUntil(offset int64) []contracts.MatchEvent {
	var out []contracts.MatchEvent
	for _, f := range p.frames {
		if f.OffsetSeconds > offset {
			break
		}
		for _, e := range f.Events {
			out = append(out, contracts.MatchEvent{
				Minute: e.Minute, Extra: e.Extra, Side: e.Side, Type: e.Type,
				Detail: e.Detail, Player: e.Player, Assist: e.Assist,
			})
		}
	}
	return out
}

type ReplayProvider struct {
	fixture contracts.ProviderFixture
	frames  []replayFrame
	speed   int64
	clock   func() time.Time
}

func NewReplayProvider(path string, speed int, clock func() time.Time) (*ReplayProvider, error) {
	if speed <= 0 {
		return nil, errs.New(errs.ErrValidation, "INVALID_REPLAY_SPEED", "la velocidad de replay debe ser positiva")
	}
	if clock == nil {
		return nil, errs.New(errs.ErrValidation, "INVALID_REPLAY_CLOCK", "el reloj de replay es obligatorio")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errs.Wrap(errs.ErrProviderUnavailable, err, "REPLAY_UNAVAILABLE", "no se pudo leer el archivo de replay")
	}
	var source replayFile
	if err := json.Unmarshal(data, &source); err != nil {
		return nil, errs.Wrap(errs.ErrProviderUnavailable, err, "REPLAY_INVALID", "el archivo de replay no contiene JSON válido")
	}
	if err := validateReplay(source); err != nil {
		return nil, err
	}

	startAt := clock().UTC()
	if source.Fixture.StartAt != nil {
		startAt = source.Fixture.StartAt.UTC()
	}
	return &ReplayProvider{
		fixture: contracts.ProviderFixture{
			ExternalID: source.Fixture.ExternalID,
			League:     source.Fixture.League,
			Home:       source.Fixture.Home,
			Away:       source.Fixture.Away,
			StartAt:    startAt,
			Status:     contracts.MatchScheduled,
		},
		frames: source.Frames,
		speed:  int64(speed),
		clock:  clock,
	}, nil
}

func (p *ReplayProvider) Name() string { return "replay" }

func (p *ReplayProvider) FixturesByDate(ctx context.Context, date time.Time, leagueIDs []string) ([]contracts.ProviderFixture, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !sameUTCDate(p.fixture.StartAt, date) {
		return nil, nil
	}
	if len(leagueIDs) > 0 && !slices.Contains(leagueIDs, p.fixture.League.ExternalID) {
		return nil, nil
	}
	return []contracts.ProviderFixture{p.fixture}, nil
}

func (p *ReplayProvider) FixtureResult(ctx context.Context, externalID string) (contracts.ProviderFixture, error) {
	if err := ctx.Err(); err != nil {
		return contracts.ProviderFixture{}, err
	}
	if externalID != p.fixture.ExternalID {
		return contracts.ProviderFixture{}, errs.New(errs.ErrNotFound, "FIXTURE_NOT_FOUND", "fixture no encontrado")
	}

	fixture := p.fixture
	frame, _, started := p.currentFrame()
	fixture.Status = contracts.MatchScheduled
	if !started {
		return fixture, nil
	}
	fixture.Status = matchStatus(frame.Status)
	fixture.HomeScore = intPointer(frame.Home)
	fixture.AwayScore = intPointer(frame.Away)
	fixture.Events = p.eventsUntil(frame.OffsetSeconds)
	return fixture, nil
}

func (p *ReplayProvider) LiveScores(ctx context.Context) ([]contracts.LiveScore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	frame, now, started := p.currentFrame()
	if !started {
		return nil, nil
	}
	status := matchStatus(frame.Status)
	if status != contracts.MatchLive && status != contracts.MatchInterrupted {
		return nil, nil
	}
	return []contracts.LiveScore{{
		ExternalID: p.fixture.ExternalID,
		Status:     status,
		HomeScore:  frame.Home,
		AwayScore:  frame.Away,
		Minute:     frame.Minute,
		ObservedAt: now,
		Events:     p.eventsUntil(frame.OffsetSeconds),
	}}, nil
}

func (p *ReplayProvider) currentFrame() (replayFrame, time.Time, bool) {
	now := p.clock().UTC()
	elapsed := now.Sub(p.fixture.StartAt)
	if elapsed < 0 {
		return replayFrame{}, now, false
	}
	elapsedSeconds := int64(elapsed / time.Second)
	lastOffset := p.frames[len(p.frames)-1].OffsetSeconds
	simulatedSeconds := lastOffset
	if elapsedSeconds <= lastOffset/p.speed {
		simulatedSeconds = elapsedSeconds * p.speed
	}

	frame := p.frames[0]
	for _, next := range p.frames[1:] {
		if next.OffsetSeconds > simulatedSeconds {
			break
		}
		frame = next
	}
	return frame, now, true
}

func validateReplay(source replayFile) error {
	fixture := source.Fixture
	if fixture.ExternalID == "" || fixture.League.ExternalID == "" || fixture.League.Name == "" ||
		fixture.Home.ExternalID == "" || fixture.Home.Name == "" ||
		fixture.Away.ExternalID == "" || fixture.Away.Name == "" {
		return invalidReplay("el fixture debe incluir IDs y nombres de liga y equipos")
	}
	if fixture.Home.ExternalID == fixture.Away.ExternalID {
		return invalidReplay("los equipos del fixture deben ser distintos")
	}
	if len(source.Frames) == 0 {
		return invalidReplay("el replay debe incluir al menos un frame")
	}

	var previous int64 = -1
	for i, frame := range source.Frames {
		if frame.OffsetSeconds < 0 || frame.OffsetSeconds < previous || frame.Minute < 0 || frame.Home < 0 || frame.Away < 0 {
			return invalidReplay(fmt.Sprintf("el frame %d contiene tiempos o marcadores inválidos", i))
		}
		if !validMatchStatus(frame.Status) {
			return invalidReplay(fmt.Sprintf("el frame %d tiene un estado desconocido", i))
		}
		previous = frame.OffsetSeconds
	}
	if !matchStatus(source.Frames[len(source.Frames)-1].Status).IsTerminal() {
		return invalidReplay("el último frame debe tener un estado terminal")
	}
	return nil
}

func invalidReplay(message string) error {
	return errs.New(errs.ErrProviderUnavailable, "REPLAY_INVALID", message)
}

func validMatchStatus(status string) bool {
	switch status {
	case "TBD", "NS", "1H", "HT", "2H", "ET", "BT", "P", "LIVE", "SUSP", "INT",
		"FT", "AET", "PEN", "PST", "CANC", "ABD", "AWD", "WO":
		return true
	default:
		return false
	}
}

func matchStatus(status string) contracts.MatchStatus {
	switch status {
	case "TBD", "NS":
		return contracts.MatchScheduled
	case "1H", "HT", "2H", "ET", "BT", "P", "LIVE":
		return contracts.MatchLive
	case "SUSP", "INT":
		return contracts.MatchInterrupted
	case "FT", "AET", "PEN":
		return contracts.MatchFinished
	case "PST":
		return contracts.MatchPostponed
	case "CANC":
		return contracts.MatchCancelled
	case "ABD":
		return contracts.MatchAbandoned
	case "AWD", "WO":
		return contracts.MatchAwarded
	default:
		return contracts.MatchScheduled
	}
}

func sameUTCDate(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

func intPointer(value int) *int { return &value }

var (
	_ contracts.FixtureProvider   = (*ReplayProvider)(nil)
	_ contracts.LiveScoreProvider = (*ReplayProvider)(nil)
)
