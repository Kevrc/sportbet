package application

import (
	"context"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

type repositoryStub struct {
	closed bool
}

func (r *repositoryStub) ListEventMarkets(context.Context, string) ([]Market, error) {
	return nil, nil
}
func (r *repositoryStub) EnsureMarkets(context.Context, string, int) error { return nil }
func (r *repositoryStub) CloseMarketsForEvent(context.Context, string) error {
	r.closed = true
	return nil
}
func (r *repositoryStub) GetSelections(context.Context, []string) (map[string]contracts.SelectionInfo, error) {
	return map[string]contracts.SelectionInfo{}, nil
}
func (r *repositoryStub) GetEventResult(context.Context, string) (contracts.EventResult, error) {
	return contracts.EventResult{Status: contracts.MatchCancelled}, nil
}
func (r *repositoryStub) ListFinishedSince(context.Context, time.Time) ([]contracts.EventResult, error) {
	return nil, nil
}
func (r *repositoryStub) ListEventSelections(context.Context, string) ([]SelectionCode, error) {
	return []SelectionCode{{ID: "selection-1", Code: "HOME"}}, nil
}

func TestResolveSelectionsVoidsCancelledEvent(t *testing.T) {
	service := NewService(&repositoryStub{}, 6)
	outcomes, err := service.ResolveSelections(context.Background(), "event-1")
	if err != nil {
		t.Fatal(err)
	}
	if outcomes["selection-1"] != contracts.OutcomeVoid {
		t.Fatalf("resultado = %q, quiere VOID", outcomes["selection-1"])
	}
}

func TestCloseMarketsForEvent(t *testing.T) {
	repository := &repositoryStub{}
	if err := NewService(repository, 6).CloseMarketsForEvent(context.Background(), "event-1"); err != nil {
		t.Fatal(err)
	}
	if !repository.closed {
		t.Fatal("no se solicitó el cierre del mercado")
	}
}
