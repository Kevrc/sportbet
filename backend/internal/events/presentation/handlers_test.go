package presentation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/events/application"
	"sportbet/internal/shared/contracts"
)

type repositoryStub struct {
	event application.Event
}

func (r *repositoryStub) ListLeagues(context.Context) ([]application.League, error) {
	return []application.League{}, nil
}
func (r *repositoryStub) ListEvents(context.Context, application.Filter) ([]application.Event, error) {
	return []application.Event{r.event}, nil
}
func (r *repositoryStub) GetEvent(context.Context, string) (application.Event, error) {
	return r.event, nil
}
func (r *repositoryStub) SetFeatured(context.Context, string, bool) (application.Event, error) {
	return r.event, nil
}
func (r *repositoryStub) ImportFixtures(context.Context, string, time.Time, []contracts.ProviderFixture) ([]application.Change, error) {
	return nil, nil
}
func (r *repositoryStub) ApplyLiveScores(context.Context, string, time.Time, []contracts.LiveScore) ([]application.Change, error) {
	return nil, nil
}

type authenticatorStub struct{}

func (authenticatorStub) RequireAuth(next http.Handler) http.Handler { return next }
func (authenticatorStub) RequireRole(...contracts.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func TestGetEventSerializesMarketAndCentesimalOdds(t *testing.T) {
	const eventID = "00000000-0000-0000-0000-000000000001"
	event := application.Event{
		ID: eventID, StartAt: time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC),
		Status: contracts.MatchScheduled,
		League: application.League{ID: "league", Name: "Liga"},
		Home:   application.Team{ID: "home", Name: "Local"},
		Away:   application.Team{ID: "away", Name: "Visitante"},
		Markets: []application.Market{{
			ID: "market", EventID: eventID, Code: contracts.MarketMatchResult, Status: contracts.MarketOpen,
			Selections: []application.Selection{{ID: "selection", Code: "HOME", Label: "Gana local", OddX100: 214}},
		}},
	}
	router := chi.NewRouter()
	Register(router, application.NewService(&repositoryStub{event: event}, nil), nil, nil, authenticatorStub{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/events/"+eventID, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{`"markets":[`, `"oddX100":214`, `"status":"SCHEDULED"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("respuesta no contiene %s: %s", expected, body)
		}
	}
}

func TestListEventsRejectsInvalidDate(t *testing.T) {
	router := chi.NewRouter()
	Register(router, application.NewService(&repositoryStub{}, nil), nil, nil, authenticatorStub{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/events?date=not-a-date", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, quiere 400; body = %s", response.Code, response.Body.String())
	}
}
