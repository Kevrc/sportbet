package application

import (
	"context"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

type repositoryStub struct {
	imported bool
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
func (r *repositoryStub) ImportFixtures(context.Context, string, time.Time, []contracts.ProviderFixture, bool) ([]Change, error) {
	r.imported = true
	return nil, nil
}
func (r *repositoryStub) ListResultCandidates(context.Context, string, time.Time) ([]string, error) {
	return nil, nil
}
func (r *repositoryStub) ApplyLiveScores(context.Context, string, time.Time, []contracts.LiveScore) ([]Change, error) {
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
