package application_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/betting/application"
	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/money"
	"sportbet/migrations"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	admin, err := db.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_betting_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	qs := u.Query()
	qs.Set("search_path", schema)
	u.RawQuery = qs.Encode()
	pool, err := db.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return pool
}

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
	pool := testPool(t)

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
	pool := testPool(t)

	markets := &mockMarket{
		selections: map[string]contracts.SelectionInfo{
			"sel-1": {SelectionID: "sel-1", EventID: "ev-1", Odd: 200, MarketStatus: contracts.MarketOpen, EventStatus: contracts.MatchScheduled, EventStartAt: time.Now().Add(time.Hour)},
			"sel-2": {SelectionID: "sel-2", EventID: "ev-1", Odd: 150, MarketStatus: contracts.MarketOpen, EventStatus: contracts.MatchScheduled, EventStartAt: time.Now().Add(time.Hour)},
		},
	}
	wallet := &mockWallet{}
	repo := &mockRepo{}

	uc := application.NewPlaceBetUseCase(pool, markets, wallet, repo, time.Now)

	_, err := uc.Execute(context.Background(), application.PlaceBetCommand{
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
