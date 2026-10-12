package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

func TestReplayFixtureResultFollowsClockedFrame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.json")
	data := `{
		"fixture": {
			"externalId": "fixture-1",
			"league": {"externalId": "league-1", "name": "Liga"},
			"home": {"externalId": "home-1", "name": "Local"},
			"away": {"externalId": "away-1", "name": "Visitante"}
		},
		"frames": [
			{"offsetSeconds": 0, "status": "NS", "home": 0, "away": 0, "minute": 0},
			{"offsetSeconds": 60, "status": "1H", "home": 1, "away": 0, "minute": 1},
			{"offsetSeconds": 120, "status": "FT", "home": 2, "away": 1, "minute": 2}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC)
	provider, err := NewReplayProvider(path, 1, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	result, err := provider.FixtureResult(context.Background(), "fixture-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != contracts.MatchScheduled || result.HomeScore == nil || *result.HomeScore != 0 {
		t.Fatalf("resultado antes del inicio = %#v", result)
	}

	now = now.Add(time.Minute)
	result, err = provider.FixtureResult(context.Background(), "fixture-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != contracts.MatchLive || result.HomeScore == nil || *result.HomeScore != 1 {
		t.Fatalf("resultado durante el replay = %#v", result)
	}

	now = now.Add(time.Minute)
	result, err = provider.FixtureResult(context.Background(), "fixture-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != contracts.MatchFinished || result.HomeScore == nil || *result.HomeScore != 2 {
		t.Fatalf("resultado al final del replay = %#v", result)
	}
}

func TestReplayLiveScoresAccumulateEventsUntilCurrentFrame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.json")
	data := `{
		"fixture": {
			"externalId": "fixture-1",
			"league": {"externalId": "league-1", "name": "Liga"},
			"home": {"externalId": "home-1", "name": "Local"},
			"away": {"externalId": "away-1", "name": "Visitante"}
		},
		"frames": [
			{"offsetSeconds": 0, "status": "NS", "home": 0, "away": 0, "minute": 0},
			{"offsetSeconds": 60, "status": "1H", "home": 1, "away": 0, "minute": 1,
			 "events": [{"minute": 1, "side": "HOME", "type": "GOAL", "detail": "Normal Goal", "player": "Rojas"}]},
			{"offsetSeconds": 120, "status": "1H", "home": 1, "away": 0, "minute": 2,
			 "events": [{"minute": 2, "side": "AWAY", "type": "CARD", "detail": "Yellow Card", "player": "Silva"}]},
			{"offsetSeconds": 180, "status": "FT", "home": 1, "away": 0, "minute": 3}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC)
	now := start
	provider, err := NewReplayProvider(path, 1, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	now = start.Add(70 * time.Second)
	scores, err := provider.LiveScores(context.Background())
	if err != nil || len(scores) != 1 || len(scores[0].Events) != 1 || scores[0].Events[0].Player != "Rojas" {
		t.Fatalf("minuto 1: scores = %+v, err = %v", scores, err)
	}
	now = start.Add(130 * time.Second)
	scores, _ = provider.LiveScores(context.Background())
	if len(scores) != 1 || len(scores[0].Events) != 2 || scores[0].Events[1].Type != contracts.MatchEventCard {
		t.Fatalf("minuto 2: deben acumularse los dos sucesos: %+v", scores)
	}
}

func TestReplayFixtureResultIncludesEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.json")
	data := `{
		"fixture": {
			"externalId": "fixture-1",
			"league": {"externalId": "league-1", "name": "Liga"},
			"home": {"externalId": "home-1", "name": "Local"},
			"away": {"externalId": "away-1", "name": "Visitante"}
		},
		"frames": [
			{"offsetSeconds": 0, "status": "NS", "home": 0, "away": 0, "minute": 0},
			{"offsetSeconds": 60, "status": "FT", "home": 1, "away": 0, "minute": 90,
			 "events": [{"minute": 90, "extra": 3, "side": "HOME", "type": "GOAL", "detail": "Normal Goal", "player": "Rojas"}]}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC)
	now := start
	provider, err := NewReplayProvider(path, 1, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	now = start.Add(2 * time.Minute)
	f, err := provider.FixtureResult(context.Background(), "fixture-1")
	if err != nil || f.Status != contracts.MatchFinished || len(f.Events) != 1 || f.Events[0].Extra != 3 {
		t.Fatalf("fixture = %+v, err = %v", f, err)
	}
}
