package domain

import (
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
)

func TestFixtureValidate(t *testing.T) {
	home, away := 2, 1
	fixture := Fixture{
		ExternalID: "provider-1",
		League:     contracts.ProviderLeague{ExternalID: "league-1", Name: "Liga"},
		Home:       contracts.ProviderTeam{ExternalID: "home-1", Name: "Local"},
		Away:       contracts.ProviderTeam{ExternalID: "away-1", Name: "Visitante"},
		StartAt:    time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC),
		Status:     contracts.MatchFinished,
		HomeScore:  &home,
		AwayScore:  &away,
	}
	if err := fixture.Validate(); err != nil {
		t.Fatalf("fixture válido rechazado: %v", err)
	}
	fixture.AwayScore = nil
	if err := fixture.Validate(); err == nil {
		t.Fatal("se aceptó un marcador incompleto")
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to contracts.MatchStatus
		admin    bool
		want     bool
	}{
		{contracts.MatchScheduled, contracts.MatchLive, false, true},
		{contracts.MatchLive, contracts.MatchFinished, false, true},
		{contracts.MatchFinished, contracts.MatchLive, false, false},
		{contracts.MatchFinished, contracts.MatchScheduled, true, true},
		{contracts.MatchFinished, contracts.MatchScheduled, false, false},
	}
	for _, tt := range tests {
		if got := CanTransition(tt.from, tt.to, tt.admin); got != tt.want {
			t.Errorf("CanTransition(%q, %q, %t) = %t, quiere %t", tt.from, tt.to, tt.admin, got, tt.want)
		}
	}
}
