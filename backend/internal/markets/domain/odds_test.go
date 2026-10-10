package domain

import (
	"math"
	"testing"

	"sportbet/internal/shared/contracts"
)

func TestCalculateOddsForAverageTeams(t *testing.T) {
	odds, err := CalculateOdds(1, 1, 6)
	if err != nil {
		t.Fatal(err)
	}
	want := [3]int64{214, 362, 314}
	for i, value := range odds.MatchResult {
		if int64(value) != want[i] {
			t.Errorf("cuota 1X2[%d] = %d, quiere %d", i, value, want[i])
		}
	}
}

func TestResolveSelection(t *testing.T) {
	tests := []struct {
		code   string
		status contracts.MatchStatus
		home   int
		away   int
		want   contracts.SelectionOutcome
	}{
		{"HOME", contracts.MatchFinished, 2, 1, contracts.OutcomeWon},
		{"DRAW", contracts.MatchFinished, 2, 1, contracts.OutcomeLost},
		{"OVER", contracts.MatchFinished, 1, 1, contracts.OutcomeLost},
		{"YES", contracts.MatchCancelled, 0, 0, contracts.OutcomeVoid},
	}
	for _, tt := range tests {
		got, err := ResolveSelection(tt.code, tt.status, tt.home, tt.away)
		if err != nil || got != tt.want {
			t.Errorf("ResolveSelection(%q, %q) = %q, %v; quiere %q", tt.code, tt.status, got, err, tt.want)
		}
	}
}

func TestCalculateOddsRejectsNonFiniteStrength(t *testing.T) {
	if _, err := CalculateOdds(math.NaN(), 1, 6); err == nil {
		t.Fatal("se aceptó una fuerza de equipo NaN")
	}
}

func TestResolveSelectionRejectsUnknownCodeForVoidedEvent(t *testing.T) {
	if _, err := ResolveSelection("UNKNOWN", contracts.MatchCancelled, 0, 0); err == nil {
		t.Fatal("se aceptó un código desconocido para un evento anulado")
	}
}
