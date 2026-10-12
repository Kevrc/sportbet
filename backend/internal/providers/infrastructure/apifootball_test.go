package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

const fixtureResponse = `{
  "errors": {},
  "response": [{
    "fixture": {"id": 123, "date": "2026-10-09T18:00:00+00:00", "status": {"short": "FT", "elapsed": 90}},
    "league": {"id": 39, "name": "Liga", "country": "Inglaterra"},
    "teams": {"home": {"id": 10, "name": "Local", "logo": "https://media.api-sports.io/football/teams/10.png", "winner": true}, "away": {"id": 20, "name": "Visitante", "logo": "https://media.api-sports.io/football/teams/20.png", "winner": false}},
    "goals": {"home": 3, "away": 2},
    "score": {"fulltime": {"home": 2, "away": 1}}
  }]
}`

func TestAPIFootballDecodesTeamResponseFields(t *testing.T) {
	var response apiResponse
	if err := json.Unmarshal([]byte(fixtureResponse), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Response) != 1 {
		t.Fatalf("fixtures = %d, want 1", len(response.Response))
	}

	home := response.Response[0].Teams.Home
	if home.ID != 10 || home.Name != "Local" || home.Logo != "https://media.api-sports.io/football/teams/10.png" {
		t.Fatalf("equipo local = %#v", home)
	}
	if home.Winner == nil || !*home.Winner {
		t.Fatalf("ganador local = %#v, want true", home.Winner)
	}
	away := response.Response[0].Teams.Away
	if away.ID != 20 || away.Name != "Visitante" || away.Logo != "https://media.api-sports.io/football/teams/20.png" {
		t.Fatalf("equipo visitante = %#v", away)
	}
	if away.Winner == nil || *away.Winner {
		t.Fatalf("ganador visitante = %#v, want false", away.Winner)
	}
}

func TestFixturesByDateUsesExternalFulltimeData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-apisports-key") != "test-key" {
			t.Errorf("falta la clave de API")
		}
		if r.URL.Query().Get("date") != "2026-10-09" {
			t.Errorf("date = %q", r.URL.Query().Get("date"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureResponse))
	}))
	defer server.Close()
	provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := provider.FixturesByDate(context.Background(), time.Date(2026, 10, 9, 4, 0, 0, 0, time.FixedZone("local", -5*60*60)), []string{"39"})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 1 || fixtures[0].HomeScore == nil || *fixtures[0].HomeScore != 2 {
		t.Fatalf("fixture = %#v", fixtures)
	}
	if fixtures[0].Status != contracts.MatchFinished {
		t.Fatalf("status = %q", fixtures[0].Status)
	}
	if fixtures[0].Home.Name != "Local" || fixtures[0].Away.Name != "Visitante" {
		t.Fatalf("equipos = %#v / %#v", fixtures[0].Home, fixtures[0].Away)
	}
}

func TestFixturesByDateFiltersLeaguesLocally(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureResponse))
	}))
	defer server.Close()
	provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := provider.FixturesByDate(context.Background(), time.Now(), []string{"999"})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 0 {
		t.Fatalf("se devolvieron ligas no solicitadas: %#v", fixtures)
	}
}

func TestLiveScoresUsesCurrentGoalsNotFinalScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("live") != "all" {
			t.Errorf("live = %q", r.URL.Query().Get("live"))
		}
		_, _ = w.Write([]byte(`{"errors":{},"response":[{
			"fixture":{"id":123,"date":"2026-10-09T18:00:00Z","status":{"short":"1H","elapsed":34}},
			"league":{"id":39,"name":"Liga","country":"Inglaterra"},
			"teams":{"home":{"id":10,"name":"Local"},"away":{"id":20,"name":"Visitante"}},
			"goals":{"home":1,"away":0},"score":{"fulltime":{"home":null,"away":null}}
		}]}`))
	}))
	defer server.Close()
	provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	scores, err := provider.LiveScores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 1 || scores[0].HomeScore != 1 || scores[0].Minute != 34 {
		t.Fatalf("marcadores = %#v", scores)
	}
}

func TestAPIFootballReturnsErrorForProviderQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.LiveScores(context.Background()); err == nil {
		t.Fatal("se ocultó el límite de cuota del proveedor")
	}
}

func TestAPIFootballRejectsInsecureRemoteURL(t *testing.T) {
	if _, err := NewAPIFootball("http://api.example.test", "test-key", nil); err == nil {
		t.Fatal("se permitió transmitir la clave por HTTP remoto")
	}
}

// La API real manda "errors": [] cuando la consulta salió bien y un objeto
// cuando falla (por ejemplo, una fecha fuera del plan gratuito).
func TestAPIFootballErrorsFieldFormats(t *testing.T) {
	cases := []struct {
		name, errors string
		wantErr      bool
	}{
		{"lista vacía (respuesta real correcta)", `[]`, false},
		{"objeto vacío", `{}`, false},
		{"objeto con error del plan", `{"plan":"Free plans do not have access to this date"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"errors":` + c.errors + `,"results":0,"response":[]}`))
			}))
			defer server.Close()
			provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.FixturesByDate(context.Background(), time.Now(), nil)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v; wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestSafeLogoURLOnlyAcceptsHTTPS(t *testing.T) {
	cases := map[string]string{
		"https://media.api-sports.io/football/teams/40.png": "https://media.api-sports.io/football/teams/40.png",
		"http://media.api-sports.io/football/teams/40.png":  "",
		"javascript:alert(1)":                               "",
		"":                                                  "",
		"https://":                                          "",
	}
	for in, want := range cases {
		if got := safeLogoURL(in); got != want {
			t.Errorf("safeLogoURL(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestFixturesByDateIncludesTeamLogos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[],"response":[{
			"fixture": {"id": 7, "date": "2026-10-11T23:30:00+00:00", "status": {"short": "NS"}},
			"league": {"id": 281, "name": "Liga 1", "country": "Peru"},
			"teams": {"home": {"id": 40, "name": "Cusco", "logo": "https://media.api-sports.io/football/teams/40.png"},
			          "away": {"id": 41, "name": "Alianza Lima", "logo": "http://inseguro.example/41.png"}},
			"goals": {"home": null, "away": null},
			"score": {"fulltime": {"home": null, "away": null}}
		}]}`))
	}))
	defer server.Close()
	provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := provider.FixturesByDate(context.Background(), time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC), nil)
	if err != nil || len(fixtures) != 1 {
		t.Fatalf("fixtures = %v, err = %v", fixtures, err)
	}
	if fixtures[0].Home.LogoURL != "https://media.api-sports.io/football/teams/40.png" || fixtures[0].Away.LogoURL != "" {
		t.Fatalf("escudos = %q / %q", fixtures[0].Home.LogoURL, fixtures[0].Away.LogoURL)
	}
}

func TestStandingsTakesRowWithMostGamesPerTeam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/standings" || r.URL.Query().Get("league") != "281" || r.URL.Query().Get("season") != "2026" {
			t.Errorf("consulta = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		// Liga 1: Apertura (14 PJ) y acumulada (30 PJ) para el mismo equipo.
		_, _ = w.Write([]byte(`{"errors":[],"response":[{"league":{"standings":[
			[{"team":{"id":2539},"all":{"played":14,"goals":{"for":20,"against":9}}}],
			[{"team":{"id":2539},"all":{"played":30,"goals":{"for":48,"against":20}}},
			 {"team":{"id":2540},"all":{"played":30,"goals":{"for":25,"against":40}}}]
		]}}]}`))
	}))
	defer server.Close()
	provider, err := NewAPIFootball(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := provider.Standings(context.Background(), "281", 2026)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	for _, r := range rows {
		if r.TeamExternalID == "2539" && (r.Played != 30 || r.GoalsFor != 48) {
			t.Fatalf("debió tomar la tabla acumulada: %+v", r)
		}
	}
}

func TestFixturesByDateIncludesLeagueAndVenueDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[],"response":[{
			"fixture": {"id": 8, "date": "2026-10-12T19:00:00+00:00", "status": {"short": "NS"},
			            "venue": {"name": "Anfield", "city": "Liverpool"}},
			"league": {"id": 39, "name": "Premier League", "country": "England", "season": 2026,
			           "logo": "https://media.api-sports.io/football/leagues/39.png",
			           "flag": "https://media.api-sports.io/flags/gb-eng.svg", "round": "Regular Season - 8"},
			"teams": {"home": {"id": 40, "name": "Liverpool"}, "away": {"id": 50, "name": "Manchester City"}},
			"goals": {"home": null, "away": null}, "score": {"fulltime": {"home": null, "away": null}}
		}]}`))
	}))
	defer server.Close()
	provider, _ := NewAPIFootball(server.URL, "test-key", server.Client())
	fixtures, err := provider.FixturesByDate(context.Background(), time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), nil)
	if err != nil || len(fixtures) != 1 {
		t.Fatalf("fixtures = %v, err = %v", fixtures, err)
	}
	f := fixtures[0]
	if f.League.Season != 2026 || f.League.LogoURL == "" || f.League.FlagURL == "" ||
		f.Round != "Regular Season - 8" || f.VenueName != "Anfield" || f.VenueCity != "Liverpool" {
		t.Fatalf("fixture = %+v", f)
	}
}

func TestStandingsFallsBackToLastSeasonAllowedByPlan(t *testing.T) {
	var seasons []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		season := r.URL.Query().Get("season")
		seasons = append(seasons, r.URL.Query().Get("league")+"@"+season)
		if season != "2024" {
			_, _ = w.Write([]byte(`{"errors":{"plan":"Free plans do not have access to this season, try from 2022 to 2024."},"response":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"errors":[],"response":[{"league":{"standings":[[{"team":{"id":40},"all":{"played":38,"goals":{"for":86,"against":41}}}]]}}]}`))
	}))
	defer server.Close()
	provider, _ := NewAPIFootball(server.URL, "test-key", server.Client())

	rows, err := provider.Standings(context.Background(), "39", 2026)
	if err != nil || len(rows) != 1 || rows[0].GoalsFor != 86 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	// La segunda liga ya va directo a 2024: una sola llamada.
	if _, err := provider.Standings(context.Background(), "140", 2026); err != nil {
		t.Fatal(err)
	}
	want := []string{"39@2026", "39@2024", "140@2024"}
	if strings.Join(seasons, ",") != strings.Join(want, ",") {
		t.Fatalf("consultas = %v; want %v", seasons, want)
	}
}

func TestLiveScoresIncludeMatchEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[],"response":[{
			"fixture": {"id": 9, "date": "2026-10-11T23:30:00+00:00", "status": {"short": "2H", "elapsed": 88}},
			"league": {"id": 128, "name": "Liga Profesional", "country": "Argentina"},
			"teams": {"home": {"id": 456, "name": "Talleres"}, "away": {"id": 453, "name": "Independiente"}},
			"goals": {"home": 1, "away": 0},
			"score": {"fulltime": {"home": null, "away": null}},
			"events": [
				{"time": {"elapsed": 8, "extra": null}, "team": {"id": 453}, "player": {"name": "Nahuel Banegas"}, "assist": {"name": null}, "type": "Card", "detail": "Yellow Card"},
				{"time": {"elapsed": 45, "extra": 2}, "team": {"id": 456}, "player": {"name": "Valoyes"}, "assist": {"name": "Girotti"}, "type": "Goal", "detail": "Normal Goal"},
				{"time": {"elapsed": 70}, "team": {"id": 456}, "player": {"name": "Depietri"}, "assist": {"name": "Botta"}, "type": "subst", "detail": "Substitution 1"},
				{"time": {"elapsed": 80}, "team": {"id": 999}, "player": {"name": "Otro"}, "type": "Goal", "detail": "Normal Goal"},
				{"time": {"elapsed": 81}, "team": {"id": 456}, "player": {"name": "X"}, "type": "Desconocido", "detail": "?"}
			]
		}]}`))
	}))
	defer server.Close()
	provider, _ := NewAPIFootball(server.URL, "test-key", server.Client())
	scores, err := provider.LiveScores(context.Background())
	if err != nil || len(scores) != 1 {
		t.Fatalf("scores = %+v, err = %v", scores, err)
	}
	ev := scores[0].Events
	if len(ev) != 3 {
		t.Fatalf("debe descartar el equipo y el tipo desconocidos: %+v", ev)
	}
	if ev[0].Type != contracts.MatchEventCard || ev[0].Side != contracts.SideAway || ev[0].Assist != "" {
		t.Fatalf("tarjeta = %+v", ev[0])
	}
	if ev[1].Type != contracts.MatchEventGoal || ev[1].Side != contracts.SideHome || ev[1].Extra != 2 || ev[1].Assist != "Girotti" {
		t.Fatalf("gol = %+v", ev[1])
	}
	if ev[2].Type != contracts.MatchEventSubst {
		t.Fatalf("cambio = %+v", ev[2])
	}
}
