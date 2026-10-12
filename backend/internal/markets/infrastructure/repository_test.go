package infrastructure_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/markets/infrastructure"
	"sportbet/internal/shared/db"
	"sportbet/migrations"
)

// testPool crea un schema propio para la prueba, aplica las migraciones y lo
// borra al terminar. Se salta si TEST_DATABASE_URL no está definida.
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
	schema := fmt.Sprintf("test_markets_%d", time.Now().UnixNano())
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

func oddOf(t *testing.T, pool *pgxpool.Pool, code string) int {
	t.Helper()
	var odd int
	if err := pool.QueryRow(context.Background(),
		`SELECT odd_x100 FROM selections WHERE code = $1`, code).Scan(&odd); err != nil {
		t.Fatal(err)
	}
	return odd
}

func TestRepriceOpenMarketsUsesCurrentStrengths(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	var eventID string
	err := pool.QueryRow(ctx, `
		WITH l AS (INSERT INTO leagues (name) VALUES ('Premier') RETURNING id),
		     h AS (INSERT INTO teams (name) VALUES ('Liverpool') RETURNING id),
		     a AS (INSERT INTO teams (name) VALUES ('Ipswich') RETURNING id)
		INSERT INTO events (league_id, home_team_id, away_team_id, start_at)
		SELECT l.id, h.id, a.id, now() + interval '3 hours' FROM l, h, a RETURNING id::text`).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureMarkets(ctx, eventID, 6); err != nil {
		t.Fatal(err)
	}
	if home := oddOf(t, pool, "HOME"); home != 214 {
		t.Fatalf("cuota inicial con fuerza 1.00 = %d; want 214", home)
	}

	if _, err := pool.Exec(ctx, `UPDATE teams SET strength = CASE name WHEN 'Liverpool' THEN 1.50 ELSE 0.70 END`); err != nil {
		t.Fatal(err)
	}
	changed, err := repo.RepriceOpenMarkets(ctx, time.Now(), 6)
	if err != nil || changed == 0 {
		t.Fatalf("changed = %d, err = %v", changed, err)
	}
	if home, away := oddOf(t, pool, "HOME"), oddOf(t, pool, "AWAY"); home >= 214 || away <= 314 {
		t.Fatalf("con Liverpool favorito: local = %d, visita = %d", home, away)
	}
	if again, _ := repo.RepriceOpenMarkets(ctx, time.Now(), 6); again != 0 {
		t.Fatalf("sin cambios de fuerza no debe recalcular (cambió %d)", again)
	}
	// Un partido que ya cerró sus mercados no se toca.
	_, _ = pool.Exec(ctx, `UPDATE markets SET status = 'CLOSED'`)
	_, _ = pool.Exec(ctx, `UPDATE teams SET strength = 1.00`)
	if n, _ := repo.RepriceOpenMarkets(ctx, time.Now(), 6); n != 0 {
		t.Fatalf("mercados cerrados recalculados: %d", n)
	}
}

func TestListEventMarketsOrdersSelectionsNaturally(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	var eventID string
	err := pool.QueryRow(ctx, `
		WITH l AS (INSERT INTO leagues (name) VALUES ('Liga') RETURNING id),
		     h AS (INSERT INTO teams (name) VALUES ('Local') RETURNING id),
		     a AS (INSERT INTO teams (name) VALUES ('Visita') RETURNING id)
		INSERT INTO events (league_id, home_team_id, away_team_id, start_at)
		SELECT l.id, h.id, a.id, now() + interval '3 hours' FROM l, h, a RETURNING id::text`).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureMarkets(ctx, eventID, 6); err != nil {
		t.Fatal(err)
	}
	markets, err := repo.ListEventMarkets(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range markets {
		codes := ""
		for _, s := range m.Selections {
			codes += s.Code + " "
		}
		got[m.Code] = codes
	}
	want := map[string]string{
		"MATCH_RESULT": "HOME DRAW AWAY ", "TOTAL_GOALS_2_5": "OVER UNDER ", "BOTH_TEAMS_TO_SCORE": "YES NO ",
	}
	for code, w := range want {
		if got[code] != w {
			t.Errorf("%s = %q; want %q", code, got[code], w)
		}
	}
}
