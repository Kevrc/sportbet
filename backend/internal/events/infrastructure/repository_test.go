package infrastructure_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/events/infrastructure"
	"sportbet/internal/shared/contracts"
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
	schema := fmt.Sprintf("test_events_%d", time.Now().UnixNano())
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

// fixtures?live=all trae partidos de ligas no importadas: no deben impedir
// que se guarde el marcador de los partidos que sí importamos.
func TestApplyLiveScoresIgnoresEventsNotImported(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	now := time.Now().UTC()
	_, err := repo.ImportFixtures(ctx, "apifootball", now, []contracts.ProviderFixture{{
		ExternalID: "100", StartAt: now.Add(-10 * time.Minute), Status: contracts.MatchScheduled,
		League: contracts.ProviderLeague{ExternalID: "281", Name: "Liga 1"},
		Home:   contracts.ProviderTeam{ExternalID: "1", Name: "Cusco"},
		Away:   contracts.ProviderTeam{ExternalID: "2", Name: "Alianza Lima"},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}

	changes, err := repo.ApplyLiveScores(ctx, "apifootball", now, []contracts.LiveScore{
		{ExternalID: "999999", Status: contracts.MatchLive, HomeScore: 3, Minute: 70, ObservedAt: now},
		{ExternalID: "100", Status: contracts.MatchLive, HomeScore: 1, AwayScore: 0, Minute: 12, ObservedAt: now},
	})
	if err != nil {
		t.Fatalf("un partido no importado no debe abortar el lote: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("cambios = %d; want 1", len(changes))
	}
	var status string
	var home, minute int
	if err := pool.QueryRow(ctx, `SELECT status, home_score, minute FROM events`).Scan(&status, &home, &minute); err != nil {
		t.Fatal(err)
	}
	if status != "LIVE" || home != 1 || minute != 12 {
		t.Fatalf("evento = %s %d min %d; want LIVE 1 min 12", status, home, minute)
	}
}

func TestImportFixturesStoresTeamLogoAndKeepsItWhenMissing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	now := time.Now().UTC()
	logo := "https://media.api-sports.io/football/teams/40.png"
	fixture := func(homeLogo string) contracts.ProviderFixture {
		return contracts.ProviderFixture{
			ExternalID: "200", StartAt: now.Add(time.Hour), Status: contracts.MatchScheduled,
			League: contracts.ProviderLeague{ExternalID: "39", Name: "Premier"},
			Home:   contracts.ProviderTeam{ExternalID: "40", Name: "Liverpool", LogoURL: homeLogo},
			Away:   contracts.ProviderTeam{ExternalID: "50", Name: "Manchester City"},
		}
	}
	if _, err := repo.ImportFixtures(ctx, "apifootball", now, []contracts.ProviderFixture{fixture(logo)}, false); err != nil {
		t.Fatal(err)
	}
	// Una importación posterior sin escudo no debe borrarlo.
	if _, err := repo.ImportFixtures(ctx, "apifootball", now, []contracts.ProviderFixture{fixture("")}, false); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM events`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	event, err := repo.GetEvent(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if event.Home.LogoURL == nil || *event.Home.LogoURL != logo {
		t.Fatalf("escudo local = %v; want %s", event.Home.LogoURL, logo)
	}
	if event.Away.LogoURL != nil {
		t.Fatalf("escudo visitante = %v; want nil", *event.Away.LogoURL)
	}
}

func TestImportStoresLeagueDetailsVenueAndStrengths(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	now := time.Now().UTC()
	_, err := repo.ImportFixtures(ctx, "apifootball", now, []contracts.ProviderFixture{{
		ExternalID: "300", StartAt: now.Add(2 * time.Hour), Status: contracts.MatchScheduled,
		League: contracts.ProviderLeague{ExternalID: "39", Name: "Premier League", Country: "England", Season: 2026,
			LogoURL: "https://media.api-sports.io/football/leagues/39.png", FlagURL: "https://media.api-sports.io/flags/gb-eng.svg"},
		Home:  contracts.ProviderTeam{ExternalID: "40", Name: "Liverpool"},
		Away:  contracts.ProviderTeam{ExternalID: "50", Name: "Manchester City"},
		Round: "Regular Season - 8", VenueName: "Anfield", VenueCity: "Liverpool",
	}}, false)
	if err != nil {
		t.Fatal(err)
	}

	var id string
	_ = pool.QueryRow(ctx, `SELECT id::text FROM events`).Scan(&id)
	event, err := repo.GetEvent(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if event.League.LogoURL == nil || event.League.FlagURL == nil || event.Round == nil || *event.Round != "Regular Season - 8" ||
		event.VenueName == nil || *event.VenueName != "Anfield" {
		t.Fatalf("evento = %+v", event)
	}

	leagues, err := repo.ListStandingsLeagues(ctx, "apifootball")
	if err != nil || len(leagues) != 1 || leagues[0].ExternalID != "39" || leagues[0].Season != 2026 {
		t.Fatalf("ligas = %+v, err = %v", leagues, err)
	}
	n, err := repo.UpdateTeamStrengths(ctx, "apifootball", map[string]float64{"40": 1.45, "50": 1.30, "999": 0.7})
	if err != nil || n != 2 {
		t.Fatalf("actualizados = %d, err = %v (el 999 no está importado)", n, err)
	}
	if n, _ := repo.UpdateTeamStrengths(ctx, "apifootball", map[string]float64{"40": 1.45}); n != 0 {
		t.Fatalf("sin cambios debe actualizar 0, actualizó %d", n)
	}
	var s float64
	_ = pool.QueryRow(ctx, `SELECT strength FROM teams WHERE name = 'Liverpool'`).Scan(&s)
	if s != 1.45 {
		t.Fatalf("fuerza = %v", s)
	}
}

func TestHasFeaturedInPlay(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	now := time.Now().UTC()
	_, err := repo.ImportFixtures(ctx, "apifootball", now, []contracts.ProviderFixture{{
		ExternalID: "400", StartAt: now.Add(-30 * time.Minute), Status: contracts.MatchLive,
		League: contracts.ProviderLeague{ExternalID: "39", Name: "Premier"},
		Home:   contracts.ProviderTeam{ExternalID: "1", Name: "A"},
		Away:   contracts.ProviderTeam{ExternalID: "2", Name: "B"},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.HasFeaturedInPlay(ctx, "apifootball", now); err != nil || ok {
		t.Fatalf("sin destacados: ok = %v, err = %v; no debe consultar en vivo", ok, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE events SET featured = true`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.HasFeaturedInPlay(ctx, "apifootball", now); !ok {
		t.Fatal("con un destacado en juego debe consultar en vivo")
	}
	if ok, _ := repo.HasFeaturedInPlay(ctx, "replay", now); ok {
		t.Fatal("solo cuenta los eventos del mismo proveedor")
	}
}

func TestApplyLiveScoresStoresMatchEvents(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := infrastructure.NewRepository(pool)
	now := time.Now().UTC()
	_, err := repo.ImportFixtures(ctx, "apifootball", now, []contracts.ProviderFixture{{
		ExternalID: "500", StartAt: now.Add(-20 * time.Minute), Status: contracts.MatchScheduled,
		League: contracts.ProviderLeague{ExternalID: "39", Name: "Premier"},
		Home:   contracts.ProviderTeam{ExternalID: "40", Name: "Liverpool"},
		Away:   contracts.ProviderTeam{ExternalID: "50", Name: "City"},
	}}, false)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	_ = pool.QueryRow(ctx, `SELECT id::text FROM events`).Scan(&id)
	goal := contracts.MatchEvent{Minute: 12, Side: contracts.SideHome, Type: contracts.MatchEventGoal, Detail: "Normal Goal", Player: "Salah", Assist: "Szoboszlai"}
	card := contracts.MatchEvent{Minute: 45, Extra: 1, Side: contracts.SideAway, Type: contracts.MatchEventCard, Detail: "Yellow Card", Player: "Rodri"}
	apply := func(minute int, events ...contracts.MatchEvent) {
		t.Helper()
		_, err := repo.ApplyLiveScores(ctx, "apifootball", now, []contracts.LiveScore{{
			ExternalID: "500", Status: contracts.MatchLive, HomeScore: 1, Minute: minute,
			ObservedAt: now.Add(time.Duration(minute) * time.Second), Events: events,
		}})
		if err != nil {
			t.Fatal(err)
		}
	}

	apply(12, goal)
	apply(46, goal, card)
	apply(47) // sin sucesos: no borra lo guardado
	event, err := repo.GetEvent(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(event.Timeline) != 2 || event.Timeline[0] != goal || event.Timeline[1] != card {
		t.Fatalf("timeline = %+v", event.Timeline)
	}
}
