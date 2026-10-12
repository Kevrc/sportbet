package application

import (
	"context"
	"errors"
	"testing"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type strengthsRepo struct {
	repositoryStub
	leagues []StandingsLeague
	saved   map[string]float64
}

func (r *strengthsRepo) ListStandingsLeagues(context.Context, string) ([]StandingsLeague, error) {
	return r.leagues, nil
}

func (r *strengthsRepo) UpdateTeamStrengths(_ context.Context, _ string, s map[string]float64) (int, error) {
	if r.saved == nil {
		r.saved = map[string]float64{}
	}
	for k, v := range s {
		r.saved[k] = v
	}
	return len(s), nil
}

type standingsStub struct {
	calls []string
	err   error
}

func (s *standingsStub) Standings(_ context.Context, league string, _ int) ([]contracts.TeamStanding, error) {
	s.calls = append(s.calls, league)
	return []contracts.TeamStanding{
		{TeamExternalID: league + "-fuerte", Played: 8, GoalsFor: 20, GoalsAgainst: 7},
		{TeamExternalID: league + "-debil", Played: 8, GoalsFor: 6, GoalsAgainst: 18},
	}, s.err
}

func TestSyncStrengthsOnlyAllowedLeagues(t *testing.T) {
	repo := &strengthsRepo{leagues: []StandingsLeague{{"39", 2026}, {"140", 2026}, {"999", 2026}}}
	provider := &standingsStub{}
	calls, updated, err := NewService(repo, nil).SyncStrengths(context.Background(), "apifootball", provider, []string{"39", "140"})
	if err != nil || calls != 2 || updated != 4 {
		t.Fatalf("calls = %d, updated = %d, err = %v", calls, updated, err)
	}
	if repo.saved["39-fuerte"] != 1.52 || repo.saved["39-debil"] != 0.65 {
		t.Fatalf("fuerzas = %v", repo.saved)
	}
}

func TestSyncStrengthsStopsOnQuota(t *testing.T) {
	repo := &strengthsRepo{leagues: []StandingsLeague{{"39", 2026}, {"140", 2026}}}
	provider := &standingsStub{err: errs.New(errs.ErrQuotaExceeded, "PROVIDER_QUOTA_EXCEEDED", "x")}
	calls, _, err := NewService(repo, nil).SyncStrengths(context.Background(), "apifootball", provider, nil)
	if !errors.Is(err, errs.ErrQuotaExceeded) || calls != 1 {
		t.Fatalf("calls = %d, err = %v", calls, err)
	}
}
