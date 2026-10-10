package application_test

import (
	"context"
	"os"
	"testing"
	"time"

	"sportbet/internal/betting/application"
	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/money"
)

type mockMarket struct {
	selections map[string]contracts.SelectionInfo
}

func (m *mockMarket) GetSelections(ctx context.Context, ids []string) (map[string]contracts.SelectionInfo, error) {
	res := make(map[string]contracts.SelectionInfo)
	for _, id := range ids {
		if s, ok := m.selections[id]; ok {
			res[id] = s
		}
	}
	return res, nil
}

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

type mockRepo struct{}

func (m *mockRepo) InsertBet(ctx context.Context, q db.Querier, bet *domain.Bet, selections []domain.BetSelection) (string, error) {
	return "bet-123", nil
}

func TestPlaceBet(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://sportbet:sportbet@localhost:5432/sportbet?sslmode=disable"
	}
	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	markets := &mockMarket{
		selections: map[string]contracts.SelectionInfo{
			"sel-1": {SelectionID: "sel-1", EventID: "ev-1", Odd: 200, MarketStatus: contracts.MarketOpen, EventStatus: contracts.MatchScheduled, EventStartAt: time.Now().Add(time.Hour)},
			"sel-2": {SelectionID: "sel-2", EventID: "ev-2", Odd: 150, MarketStatus: contracts.MarketOpen, EventStatus: contracts.MatchScheduled, EventStartAt: time.Now().Add(time.Hour)},
		},
	}
	wallet := &mockWallet{}
	repo := &mockRepo{}

	uc := application.NewPlaceBetUseCase(pool, markets, wallet, repo, time.Now)

	res, err := uc.Execute(context.Background(), application.PlaceBetCommand{
		UserID:         "user-1",
		IdempotencyKey: "idem-1",
		Stake:          money.FromCents(1000), // 10.00
		Selections: []application.SelectionInput{
			{SelectionID: "sel-1", Odd: 200},
			{SelectionID: "sel-2", Odd: 150},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.BetID != "bet-123" {
		t.Errorf("expected betID bet-123, got %s", res.BetID)
	}
	if res.TotalOdd != 300 { // 2.00 * 1.50 = 3.00
		t.Errorf("expected total odd 300, got %d", res.TotalOdd)
	}
	if res.PotentialPayout.Cents() != 3000 { // 1000 * 3.00
		t.Errorf("expected potential payout 3000, got %d", res.PotentialPayout.Cents())
	}
}

func TestPlaceBet_Correlated(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://sportbet:sportbet@localhost:5432/sportbet?sslmode=disable"
	}
	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	markets := &mockMarket{
		selections: map[string]contracts.SelectionInfo{
			"sel-1": {SelectionID: "sel-1", EventID: "ev-1", Odd: 200, MarketStatus: contracts.MarketOpen, EventStatus: contracts.MatchScheduled, EventStartAt: time.Now().Add(time.Hour)},
			"sel-2": {SelectionID: "sel-2", EventID: "ev-1", Odd: 150, MarketStatus: contracts.MarketOpen, EventStatus: contracts.MatchScheduled, EventStartAt: time.Now().Add(time.Hour)},
		},
	}
	wallet := &mockWallet{}
	repo := &mockRepo{}

	uc := application.NewPlaceBetUseCase(pool, markets, wallet, repo, time.Now)

	_, err = uc.Execute(context.Background(), application.PlaceBetCommand{
		UserID:         "user-1",
		IdempotencyKey: "idem-2",
		Stake:          money.FromCents(1000),
		Selections: []application.SelectionInput{
			{SelectionID: "sel-1", Odd: 200},
			{SelectionID: "sel-2", Odd: 150},
		},
	})

	if err != domain.ErrCorrelatedSelections {
		t.Fatalf("expected ErrCorrelatedSelections, got: %v", err)
	}
}
