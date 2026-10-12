package infrastructure

import (
	"context"
	"sync"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type ProviderManager struct {
	mu       sync.RWMutex
	fixtures contracts.FixtureProvider
	live     contracts.LiveScoreProvider
}

func NewProviderManager(fixtures contracts.FixtureProvider, live contracts.LiveScoreProvider) *ProviderManager {
	return &ProviderManager{fixtures: fixtures, live: live}
}

func (m *ProviderManager) Name() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fixtures.Name()
}

func (m *ProviderManager) FixturesByDate(ctx context.Context, date time.Time, leagues []string) ([]contracts.ProviderFixture, error) {
	m.mu.RLock()
	provider := m.fixtures
	m.mu.RUnlock()
	return provider.FixturesByDate(ctx, date, leagues)
}

func (m *ProviderManager) FixtureResult(ctx context.Context, id string) (contracts.ProviderFixture, error) {
	m.mu.RLock()
	provider := m.fixtures
	m.mu.RUnlock()
	return provider.FixtureResult(ctx, id)
}

func (m *ProviderManager) LiveScores(ctx context.Context) ([]contracts.LiveScore, error) {
	m.mu.RLock()
	provider := m.live
	m.mu.RUnlock()
	return provider.LiveScores(ctx)
}

func (m *ProviderManager) StartReplay(path string, speed int, clock func() time.Time) error {
	replay, err := NewReplayProvider(path, speed, clock)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.fixtures, m.live = replay, replay
	m.mu.Unlock()
	return nil
}

type unavailableProvider struct {
	cause error
}

func NewUnavailableProvider(cause error) *unavailableProvider {
	return &unavailableProvider{cause: cause}
}

func (p *unavailableProvider) Name() string { return "unavailable" }
func (p *unavailableProvider) FixturesByDate(context.Context, time.Time, []string) ([]contracts.ProviderFixture, error) {
	return nil, errs.Wrap(errs.ErrProviderUnavailable, p.cause, "PROVIDER_UNAVAILABLE", "el proveedor deportivo no está disponible")
}
func (p *unavailableProvider) FixtureResult(context.Context, string) (contracts.ProviderFixture, error) {
	return contracts.ProviderFixture{}, errs.Wrap(errs.ErrProviderUnavailable, p.cause, "PROVIDER_UNAVAILABLE", "el proveedor deportivo no está disponible")
}
func (p *unavailableProvider) LiveScores(context.Context) ([]contracts.LiveScore, error) {
	return nil, errs.Wrap(errs.ErrProviderUnavailable, p.cause, "PROVIDER_UNAVAILABLE", "el proveedor deportivo no está disponible")
}

var (
	_ contracts.FixtureProvider   = (*ProviderManager)(nil)
	_ contracts.LiveScoreProvider = (*ProviderManager)(nil)
)

// Standings delega en el proveedor actual si entrega tablas de posiciones.
func (m *ProviderManager) Standings(ctx context.Context, leagueExternalID string, season int) ([]contracts.TeamStanding, error) {
	m.mu.RLock()
	provider := m.fixtures
	m.mu.RUnlock()
	standings, ok := provider.(contracts.StandingsProvider)
	if !ok {
		return nil, errs.New(errs.ErrProviderUnavailable, "STANDINGS_UNSUPPORTED", "el proveedor actual no entrega tablas de posiciones")
	}
	return standings.Standings(ctx, leagueExternalID, season)
}
