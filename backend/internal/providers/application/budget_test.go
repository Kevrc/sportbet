package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type usageStub struct {
	limits []int
}

func (u *usageStub) Consume(_ context.Context, _ string, _ time.Time, limit int) error {
	u.limits = append(u.limits, limit)
	if len(u.limits) > 1 && limit == 5 {
		return errs.New(errs.ErrQuotaExceeded, "DAILY_PROVIDER_BUDGET", "daily budget exhausted")
	}
	return nil
}
func (u *usageStub) RequestsUsed(context.Context, string, time.Time) (int, error) { return 4, nil }

type fixtureStub struct{}

func (fixtureStub) Name() string { return "apifootball" }
func (fixtureStub) FixturesByDate(context.Context, time.Time, []string) ([]contracts.ProviderFixture, error) {
	return nil, nil
}
func (fixtureStub) FixtureResult(context.Context, string) (contracts.ProviderFixture, error) {
	return contracts.ProviderFixture{}, nil
}

type liveStub struct{ fixtureStub }

func (liveStub) LiveScores(context.Context) ([]contracts.LiveScore, error) { return nil, nil }

func TestBudgetReservesRequestsFromFixtureCalls(t *testing.T) {
	usage := &usageStub{}
	budget, err := NewBudget(usage, 10, 5, func() time.Time { return time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	fixtures := NewBudgetedFixtures(fixtureStub{}, budget)
	_, err = fixtures.FixturesByDate(context.Background(), time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	live := NewBudgetedLiveScores(liveStub{}, budget)
	_, err = live.LiveScores(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(usage.limits) != 2 || usage.limits[0] != 5 || usage.limits[1] != 10 {
		t.Fatalf("límites reservados = %v, quiere [5 10]", usage.limits)
	}
}

func TestNewBudgetRejectsInvalidConfiguration(t *testing.T) {
	_, err := NewBudget(&usageStub{}, 0, 0, time.Now)
	if !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("error = %v, quiere error de validación", err)
	}
}
