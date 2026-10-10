package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

func TestStartReplayUsesInjectedClockAndReplacesUnavailableProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.json")
	fixture := `{
		"fixture": {
			"externalId": "fixture-1",
			"league": {"externalId": "league-1", "name": "Liga"},
			"home": {"externalId": "home-1", "name": "Local"},
			"away": {"externalId": "away-1", "name": "Visitante"},
			"startAt": null
		},
		"frames": [{"offsetSeconds": 0, "status": "FT", "home": 1, "away": 0, "minute": 90}]
	}`
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	unavailable := NewUnavailableProvider(errors.New("not configured"))
	manager := NewProviderManager(unavailable, unavailable)
	start := time.Date(2026, 10, 9, 18, 30, 0, 0, time.FixedZone("local", -5*60*60))
	if err := manager.StartReplay(path, 1, func() time.Time { return start }); err != nil {
		t.Fatal(err)
	}
	fixtures, err := manager.FixturesByDate(context.Background(), start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if manager.Name() != "replay" || len(fixtures) != 1 || !fixtures[0].StartAt.Equal(start.UTC()) {
		t.Fatalf("provider=%q fixtures=%#v", manager.Name(), fixtures)
	}
	if fixtures[0].Status != contracts.MatchScheduled {
		t.Fatalf("status = %q", fixtures[0].Status)
	}
}

func TestStartReplayDoesNotReplaceProviderOnInvalidFile(t *testing.T) {
	unavailable := NewUnavailableProvider(errors.New("not configured"))
	manager := NewProviderManager(unavailable, unavailable)
	if err := manager.StartReplay(filepath.Join(t.TempDir(), "missing.json"), 1, time.Now); err == nil {
		t.Fatal("se aceptó un archivo inexistente")
	}
	if manager.Name() != "unavailable" {
		t.Fatalf("el proveedor cambió tras un inicio fallido: %q", manager.Name())
	}
}
