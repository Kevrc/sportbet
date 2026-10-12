package infrastructure_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/admin/application"
	"sportbet/internal/admin/infrastructure"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/eventbus"
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
	schema := fmt.Sprintf("test_admin_%d", time.Now().UnixNano())
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

// El flujo completo: el bus entrega los anuncios de apuestas y el panel los
// cuenta.
func TestBusEventsFeedAuditAndStats(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	var ana, beto string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, display_name) VALUES ('ana@x.com','h','Ana') RETURNING id::text`).Scan(&ana); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash, display_name) VALUES ('beto@x.com','h','Beto') RETURNING id::text`).Scan(&beto); err != nil {
		t.Fatal(err)
	}

	svc := application.NewService(infrastructure.NewRepository(pool))
	now := time.Now()
	steps := []error{
		svc.OnBetPlaced(ctx, eventbus.BetPlaced{BetID: "b1", UserID: ana, Stake: 1000, TotalOdd: 214, Selections: 1, PlacedAt: now}),
		svc.OnBetPlaced(ctx, eventbus.BetPlaced{BetID: "b2", UserID: beto, Stake: 500, TotalOdd: 362, Selections: 1, PlacedAt: now}),
		svc.OnBetSettled(ctx, eventbus.BetSettled{BetID: "b1", UserID: ana, Status: contracts.BetWon, Payout: 2140, SettledAt: now}),
		svc.OnBetSettled(ctx, eventbus.BetSettled{BetID: "b2", UserID: beto, Status: contracts.BetLost, Payout: 0, SettledAt: now}),
	}
	for _, err := range steps {
		if err != nil {
			t.Fatal(err)
		}
	}

	s, err := svc.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.Users != 2 || s.BetsPlaced != 2 || s.BetsSettled != 2 || s.StakeCents != 1500 || s.PayoutCents != 2140 {
		t.Fatalf("stats = %+v", s)
	}

	entries, total, err := svc.List(ctx, 3, 0)
	if err != nil || total != 4 || len(entries) != 3 {
		t.Fatalf("entries = %d, total = %d, err = %v", len(entries), total, err)
	}
	last := entries[0]
	if last.Action != "BET_SETTLED" || last.ActorName == nil || *last.ActorName != "Beto" || *last.EntityID != "b2" {
		t.Fatalf("más reciente = %+v", last)
	}
}
