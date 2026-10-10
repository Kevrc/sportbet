package presentation

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
)

type usageStub struct{}

func (usageStub) RequestsUsed(context.Context, string, time.Time) (int, error) { return 4, nil }

type fixtureProviderStub struct {
	startedPath  string
	startedSpeed int
}

func (*fixtureProviderStub) Name() string { return "apifootball" }
func (*fixtureProviderStub) FixturesByDate(context.Context, time.Time, []string) ([]contracts.ProviderFixture, error) {
	return nil, nil
}
func (*fixtureProviderStub) FixtureResult(context.Context, string) (contracts.ProviderFixture, error) {
	return contracts.ProviderFixture{}, nil
}
func (p *fixtureProviderStub) StartReplay(path string, speed int, _ func() time.Time) error {
	p.startedPath, p.startedSpeed = path, speed
	return nil
}

type authenticatorStub struct{}

func (authenticatorStub) RequireAuth(next http.Handler) http.Handler { return next }
func (authenticatorStub) RequireRole(...contracts.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func testRouter(cfg config.Config, provider *fixtureProviderStub) *chi.Mux {
	router := chi.NewRouter()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	Register(router, cfg, func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC) },
		authenticatorStub{}, provider, usageStub{}, log)
	return router
}

func TestProviderStatusReturnsPersistedUsageAndBudget(t *testing.T) {
	provider := &fixtureProviderStub{}
	cfg := config.Config{Provider: "apifootball", ProviderDailyBudget: 100}
	response := httptest.NewRecorder()
	testRouter(cfg, provider).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/providers/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, value := range []string{`"provider":"apifootball"`, `"day":"2026-10-09"`, `"requestsUsed":4`, `"budget":100`} {
		if !strings.Contains(response.Body.String(), value) {
			t.Errorf("respuesta no contiene %s: %s", value, response.Body.String())
		}
	}
}

func TestStartReplayRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	configured := filepath.Join(root, "sample.json")
	if err := os.WriteFile(configured, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := &fixtureProviderStub{}
	cfg := config.Config{ReplayFile: configured, ReplaySpeed: 1}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/replay/start", strings.NewReader(`{"file":"..\\outside.json"}`))
	testRouter(cfg, provider).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if provider.startedPath != "" {
		t.Fatal("se inició un archivo fuera del directorio permitido")
	}
}

func TestStartReplayUsesOnlyAValidatedFileInConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	configured := filepath.Join(root, "default.json")
	selected := filepath.Join(root, "selected.json")
	for _, path := range []string{configured, selected} {
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	provider := &fixtureProviderStub{}
	cfg := config.Config{ReplayFile: configured, ReplaySpeed: 2}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/replay/start", strings.NewReader(`{"file":"selected.json","speed":4}`))
	testRouter(cfg, provider).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if provider.startedPath != selected || provider.startedSpeed != 4 {
		t.Fatalf("replay = (%q, %d)", provider.startedPath, provider.startedSpeed)
	}
}
