package infrastructure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

const fixtureResponse = `{
  "errors": {},
  "response": [{
    "fixture": {"id": 123, "date": "2026-10-09T18:00:00+00:00", "status": {"short": "FT", "elapsed": 90}},
    "league": {"id": 39, "name": "Liga", "country": "Inglaterra"},
    "teams": {"home": {"id": 10, "name": "Local"}, "away": {"id": 20, "name": "Visitante"}},
    "goals": {"home": 3, "away": 2},
    "score": {"fulltime": {"home": 2, "away": 1}}
  }]
}`

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
