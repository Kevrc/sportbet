package application_test

import (
	"context"
	"testing"
	"time"

	"sportbet/internal/settlement/application"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/money"
)

type mockWallet struct{}

func (m *mockWallet) CreateWallet(ctx context.Context, q db.Querier, userID string, initial money.Money) error {
	return nil
}
func (m *mockWallet) Debit(ctx context.Context, q db.Querier, userID string, amt money.Money, ref contracts.TxRef) (money.Money, error) {
	return money.FromCents(0), nil
}
func (m *mockWallet) Credit(ctx context.Context, q db.Querier, userID string, amt money.Money, ref contracts.TxRef) (money.Money, error) {
	return money.FromCents(0), nil
}
func (m *mockWallet) Balance(ctx context.Context, userID string) (money.Money, error) {
	return money.FromCents(0), nil
}

type mockResults struct{}

func (m *mockResults) GetResult(ctx context.Context, eventID string) (contracts.EventResult, error) {
	return contracts.EventResult{}, nil
}
func (m *mockResults) ResolveSelections(ctx context.Context, eventID string) (map[string]contracts.SelectionOutcome, error) {
	return map[string]contracts.SelectionOutcome{
		"sel-1": contracts.OutcomeWon,
	}, nil
}
func (m *mockResults) ListFinishedSince(ctx context.Context, since time.Time) ([]contracts.EventResult, error) {
	return []contracts.EventResult{{EventID: "ev-1"}}, nil
}

func TestSettlerSweep(t *testing.T) {
	// Simple test just to prove logic initializes without panic
	wallet := &mockWallet{}
	results := &mockResults{}

	settler := application.NewSettler(nil, wallet, results, nil, time.Now, nil)
	// Only test if it exists
	if settler == nil {
		t.Fatal("expected settler to be initialized")
	}
}
