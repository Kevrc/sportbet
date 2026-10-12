package application

import (
	"context"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

type repositoryStub struct {
	imported   bool
	candidates []ResultCandidate
	refreshed  []string
	live       []contracts.LiveScore
}

func (r *repositoryStub) ListLeagues(context.Context) ([]League, error) { return nil, nil }
func (r *repositoryStub) ListEvents(context.Context, Filter) ([]Event, error) {
	return nil, nil
}
func (r *repositoryStub) GetEvent(context.Context, string) (Event, error) {
	return Event{}, nil
}
func (r *repositoryStub) SetFeatured(context.Context, string, bool) (Event, error) {
	return Event{}, nil
}
func (r *repositoryStub) ImportFixtures(_ context.Context, _ string, _ time.Time, fixtures []contracts.ProviderFixture, refresh bool) ([]Change, error) {
	r.imported = true
	if refresh {
		for _, f := range fixtures {
			r.refreshed = append(r.refreshed, f.ExternalID)
		}
	}
	return nil, nil
}
func (r *repositoryStub) ListResultCandidates(context.Context, string, time.Time) ([]ResultCandidate, error) {
	return r.candidates, nil
}
func (r *repositoryStub) ApplyLiveScores(_ context.Context, _ string, _ time.Time, scores []contracts.LiveScore) ([]Change, error) {
	r.live = scores
	return nil, nil
}

func TestImportFixturesRejectsInvalidFixture(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, nil)
	if err := service.ImportFixtures(context.Background(), "replay", time.Now(), []contracts.ProviderFixture{{}}); err == nil {
		t.Fatal("se aceptó un fixture incompleto")
	}
	if repository.imported {
		t.Fatal("se persistió un fixture inválido")
	}
}

func TestListEventsRejectsUnknownStatus(t *testing.T) {
	service := NewService(&repositoryStub{}, nil)
	if _, err := service.ListEvents(context.Background(), Filter{Status: "UNKNOWN"}); err == nil {
		t.Fatal("se aceptó un estado desconocido")
	}
}

func (r *repositoryStub) ListStandingsLeagues(context.Context, string) ([]StandingsLeague, error) {
	return nil, nil
}
func (r *repositoryStub) UpdateTeamStrengths(context.Context, string, map[string]float64) (int, error) {
	return 0, nil
}
func (r *repositoryStub) HasFeaturedInPlay(context.Context, string, time.Time) (bool, error) {
	return false, nil
}
func (r *repositoryStub) ListTimelinePending(context.Context, string, time.Time, int) ([]string, error) {
	return nil, nil
}
func (r *repositoryStub) MarkTimelineFetched(context.Context, string, string, time.Time) error {
	return nil
}
