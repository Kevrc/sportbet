package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type fixturesStub struct {
	byDate    map[string][]contracts.ProviderFixture
	dateCalls []string
	idCalls   []string
	dateErr   error
}

func (f *fixturesStub) Name() string { return "apifootball" }

func (f *fixturesStub) FixturesByDate(_ context.Context, date time.Time, _ []string) ([]contracts.ProviderFixture, error) {
	day := date.UTC().Format("2006-01-02")
	f.dateCalls = append(f.dateCalls, day)
	return f.byDate[day], f.dateErr
}

func (f *fixturesStub) FixtureResult(_ context.Context, id string) (contracts.ProviderFixture, error) {
	f.idCalls = append(f.idCalls, id)
	return fixture(id, time.Date(2026, 10, 8, 15, 0, 0, 0, time.UTC)), nil
}

func fixture(id string, start time.Time) contracts.ProviderFixture {
	two, one := 2, 1
	return contracts.ProviderFixture{
		ExternalID: id, StartAt: start, Status: contracts.MatchFinished, HomeScore: &two, AwayScore: &one,
		League: contracts.ProviderLeague{ExternalID: "39", Name: "Premier"},
		Home:   contracts.ProviderTeam{ExternalID: "h" + id, Name: "Local " + id},
		Away:   contracts.ProviderTeam{ExternalID: "a" + id, Name: "Visita " + id},
	}
}

func TestConfirmResultsGroupsByDate(t *testing.T) {
	now := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	d11 := time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)
	d12 := time.Date(2026, 10, 12, 0, 30, 0, 0, time.UTC)
	repo := &repositoryStub{candidates: []ResultCandidate{
		{"1", d11}, {"2", d11}, {"3", d11}, {"4", d12},
	}}
	provider := &fixturesStub{byDate: map[string][]contracts.ProviderFixture{
		"2026-10-11": {fixture("1", d11), fixture("2", d11), fixture("3", d11), fixture("99", d11)},
		"2026-10-12": {fixture("4", d12)},
	}}
	calls, err := NewService(repo, nil).ConfirmResults(context.Background(), "apifootball", provider, nil, now, now, true)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(provider.idCalls) != 0 {
		t.Fatalf("calls = %d, por fecha = %v, por partido = %v; want 2 por fecha", calls, provider.dateCalls, provider.idCalls)
	}
	slices.Sort(repo.refreshed)
	if !slices.Equal(repo.refreshed, []string{"1", "2", "3", "4"}) {
		t.Fatalf("refrescados = %v; el 99 no era candidato", repo.refreshed)
	}
}

func TestConfirmResultsOldDatesFallBackToSingleFixture(t *testing.T) {
	now := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	repo := &repositoryStub{candidates: []ResultCandidate{{"7", time.Date(2026, 10, 10, 15, 0, 0, 0, time.UTC)}}}
	provider := &fixturesStub{}
	calls, err := NewService(repo, nil).ConfirmResults(context.Background(), "apifootball", provider, nil, now, now, true)
	if err != nil || calls != 1 || len(provider.dateCalls) != 0 || !slices.Equal(provider.idCalls, []string{"7"}) {
		t.Fatalf("calls = %d, err = %v, fecha = %v, id = %v", calls, err, provider.dateCalls, provider.idCalls)
	}
}

func TestConfirmResultsStopsOnQuota(t *testing.T) {
	now := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	repo := &repositoryStub{candidates: []ResultCandidate{
		{"1", time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)},
		{"2", time.Date(2026, 10, 12, 0, 30, 0, 0, time.UTC)},
	}}
	provider := &fixturesStub{dateErr: errs.New(errs.ErrQuotaExceeded, "PROVIDER_QUOTA_EXCEEDED", "x")}
	calls, err := NewService(repo, nil).ConfirmResults(context.Background(), "apifootball", provider, nil, now, now, true)
	if !errors.Is(err, errs.ErrQuotaExceeded) || calls != 1 {
		t.Fatalf("calls = %d, err = %v; debe detenerse tras la primera llamada sin cuota", calls, err)
	}
}

func TestConfirmResultsWithoutCandidatesMakesNoCalls(t *testing.T) {
	provider := &fixturesStub{}
	calls, err := NewService(&repositoryStub{}, nil).ConfirmResults(context.Background(), "apifootball", provider, nil, time.Now(), time.Now(), true)
	if err != nil || calls != 0 || len(provider.dateCalls)+len(provider.idCalls) != 0 {
		t.Fatalf("calls = %d, err = %v", calls, err)
	}
}

func TestApplyLiveScoresSkipsInvalidScoresInsteadOfFailing(t *testing.T) {
	repo := &repositoryStub{}
	scores := []contracts.LiveScore{
		{ExternalID: "1", Status: contracts.MatchLive, HomeScore: 1},
		{ExternalID: "", Status: contracts.MatchLive},
		{ExternalID: "3", Status: contracts.MatchFinished},
	}
	if err := NewService(repo, nil).ApplyLiveScores(context.Background(), "apifootball", time.Now(), scores); err != nil {
		t.Fatal(err)
	}
	if len(repo.live) != 1 || repo.live[0].ExternalID != "1" {
		t.Fatalf("marcadores guardados = %+v; want solo el válido", repo.live)
	}
}

func TestApplyLiveScoresDropsInvalidMatchEvents(t *testing.T) {
	repo := &repositoryStub{}
	scores := []contracts.LiveScore{{ExternalID: "1", Status: contracts.MatchLive, Events: []contracts.MatchEvent{
		{Minute: 10, Side: contracts.SideHome, Type: contracts.MatchEventGoal},
		{Minute: 11, Side: "LOCAL", Type: contracts.MatchEventGoal},
		{Minute: 12, Side: contracts.SideAway, Type: "PENALTY"},
		{Minute: -1, Side: contracts.SideAway, Type: contracts.MatchEventCard},
	}}}
	if err := NewService(repo, nil).ApplyLiveScores(context.Background(), "apifootball", time.Now(), scores); err != nil {
		t.Fatal(err)
	}
	if len(repo.live) != 1 || len(repo.live[0].Events) != 1 || repo.live[0].Events[0].Minute != 10 {
		t.Fatalf("sucesos guardados = %+v", repo.live)
	}
}
