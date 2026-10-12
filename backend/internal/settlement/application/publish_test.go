package application_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/settlement/application"
	"sportbet/internal/settlement/infrastructure"
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
	schema := fmt.Sprintf("test_settlement_%d", time.Now().UnixNano())
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

func TestSettleBetPublishesBetSettledOnce(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	var userID, betID string
	err := pool.QueryRow(ctx, `
		WITH u AS (INSERT INTO users (email, password_hash, display_name) VALUES ('ana@x.com', 'h', 'Ana') RETURNING id),
		     l AS (INSERT INTO leagues (name) VALUES ('Liga') RETURNING id),
		     h AS (INSERT INTO teams (name) VALUES ('Local') RETURNING id),
		     a AS (INSERT INTO teams (name) VALUES ('Visita') RETURNING id),
		     e AS (INSERT INTO events (league_id, home_team_id, away_team_id, start_at, status)
		           SELECT l.id, h.id, a.id, now() - interval '2 hours', 'FINISHED' FROM l, h, a RETURNING id),
		     m AS (INSERT INTO markets (event_id, code) SELECT id, 'MATCH_RESULT' FROM e RETURNING id),
		     s AS (INSERT INTO selections (market_id, code, label, odd_x100, result)
		           SELECT id, 'HOME', 'Gana local', 214, 'WON' FROM m RETURNING id),
		     b AS (INSERT INTO bets (user_id, idempotency_key, stake_cents, total_odd_x100, potential_payout_cents)
		           SELECT id, 'k1', 1000, 214, 2140 FROM u RETURNING id, user_id),
		     bs AS (INSERT INTO bet_selections (bet_id, selection_id, event_id, odd_x100, result)
		            SELECT b.id, s.id, e.id, 214, 'WON' FROM b, s, e RETURNING bet_id)
		SELECT b.user_id::text, b.id::text FROM b, bs`).Scan(&userID, &betID)
	if err != nil {
		t.Fatal(err)
	}

	bus := eventbus.NewRecorder()
	settler := application.NewSettler(pool, &mockWallet{}, &mockResults{}, infrastructure.NewRepository(pool), time.Now, bus)
	if err := settler.SettleBet(ctx, betID); err != nil {
		t.Fatal(err)
	}
	if err := settler.SettleBet(ctx, betID); err != nil {
		t.Fatal(err)
	}

	events := bus.Events()
	if len(events) != 1 {
		t.Fatalf("eventos = %+v; una apuesta se anuncia una sola vez", events)
	}
	settled, ok := events[0].(eventbus.BetSettled)
	if !ok || settled.BetID != betID || settled.UserID != userID ||
		settled.Status != contracts.BetWon || settled.Payout != 2140 {
		t.Fatalf("BetSettled = %+v", events[0])
	}
}
