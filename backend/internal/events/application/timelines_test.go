package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type timelinesRepo struct {
	repositoryStub
	pending []string
	limit   int
	marked  []string
}

func (r *timelinesRepo) ListTimelinePending(_ context.Context, _ string, _ time.Time, limit int) ([]string, error) {
	r.limit = limit
	return r.pending, nil
}

func (r *timelinesRepo) MarkTimelineFetched(_ context.Context, _, id string, _ time.Time) error {
	r.marked = append(r.marked, id)
	return nil
}

type resultStub struct {
	fixturesStub
	failOn map[string]error
}

func (f *resultStub) FixtureResult(ctx context.Context, id string) (contracts.ProviderFixture, error) {
	if err := f.failOn[id]; err != nil {
		f.idCalls = append(f.idCalls, id)
		return contracts.ProviderFixture{}, err
	}
	return f.fixturesStub.FixtureResult(ctx, id)
}

func TestFillTimelinesFetchesEachPendingMatchOnce(t *testing.T) {
	repo := &timelinesRepo{pending: []string{"1", "2"}}
	provider := &resultStub{}
	calls, err := NewService(repo, nil).FillTimelines(context.Background(), "apifootball", provider, time.Now(), 3)
	if err != nil || calls != 2 || repo.limit != 3 {
		t.Fatalf("calls = %d, limit = %d, err = %v", calls, repo.limit, err)
	}
	if !slices.Equal(repo.refreshed, []string{"1", "2"}) || !slices.Equal(repo.marked, []string{"1", "2"}) {
		t.Fatalf("refrescados = %v, marcados = %v", repo.refreshed, repo.marked)
	}
}

func TestFillTimelinesStopsOnQuotaAndDoesNotMarkFailures(t *testing.T) {
	repo := &timelinesRepo{pending: []string{"1", "2", "3"}}
	provider := &resultStub{failOn: map[string]error{"2": errs.New(errs.ErrQuotaExceeded, "PROVIDER_QUOTA_EXCEEDED", "x")}}
	calls, err := NewService(repo, nil).FillTimelines(context.Background(), "apifootball", provider, time.Now(), 3)
	if !errors.Is(err, errs.ErrQuotaExceeded) || calls != 2 || !slices.Equal(repo.marked, []string{"1"}) {
		t.Fatalf("calls = %d, marcados = %v, err = %v", calls, repo.marked, err)
	}
}

func TestFillTimelinesWithZeroMaxDoesNothing(t *testing.T) {
	repo := &timelinesRepo{pending: []string{"1"}}
	if calls, err := NewService(repo, nil).FillTimelines(context.Background(), "apifootball", &resultStub{}, time.Now(), 0); calls != 0 || err != nil {
		t.Fatalf("calls = %d, err = %v", calls, err)
	}
}
