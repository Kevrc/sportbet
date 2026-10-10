package domain

import (
	"errors"
	"testing"
)

func TestDailyBudgetReservesCapacityForLiveScores(t *testing.T) {
	budget, err := NewDailyBudget(100, 13)
	if err != nil {
		t.Fatal(err)
	}
	if got := budget.Limit(FixtureRequest); got != 87 {
		t.Fatalf("límite fixtures = %d, quiere 87", got)
	}
	if got := budget.Limit(LiveScoreRequest); got != 100 {
		t.Fatalf("límite live = %d, quiere 100", got)
	}
}

func TestDailyBudgetRejectsInvalidLimits(t *testing.T) {
	for _, limits := range [][2]int{{0, 0}, {10, -1}, {10, 10}} {
		if _, err := NewDailyBudget(limits[0], limits[1]); !errors.Is(err, ErrInvalidDailyBudget) {
			t.Errorf("NewDailyBudget(%d, %d) error = %v", limits[0], limits[1], err)
		}
	}
}
