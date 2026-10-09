package application

import (
	"context"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type UsageRepository interface {
	Consume(context.Context, string, time.Time, int) error
	RequestsUsed(context.Context, string, time.Time) (int, error)
}

type Budget struct {
	repository UsageRepository
	dailyLimit int
	reserve    int
	clock      func() time.Time
}

func NewBudget(repository UsageRepository, dailyLimit, reserve int, clock func() time.Time) (*Budget, error) {
	if repository == nil || clock == nil || dailyLimit <= 0 || reserve < 0 || reserve >= dailyLimit {
		return nil, errs.New(errs.ErrValidation, "INVALID_PROVIDER_BUDGET", "la configuración del presupuesto diario es inválida")
	}
	return &Budget{repository: repository, dailyLimit: dailyLimit, reserve: reserve, clock: clock}, nil
}

func (b *Budget) Allow(ctx context.Context, provider string, reserveProtected bool) error {
	limit := b.dailyLimit
	if reserveProtected {
		limit -= b.reserve
	}
	if err := b.repository.Consume(ctx, provider, b.clock().UTC(), limit); err != nil {
		return err
	}
	return nil
}

func (b *Budget) RequestsUsed(ctx context.Context, provider string) (int, error) {
	return b.repository.RequestsUsed(ctx, provider, b.clock().UTC())
}

func (b *Budget) DailyLimit() int { return b.dailyLimit }

type BudgetedFixtures struct {
	provider contracts.FixtureProvider
	budget   *Budget
}

func NewBudgetedFixtures(provider contracts.FixtureProvider, budget *Budget) *BudgetedFixtures {
	return &BudgetedFixtures{provider: provider, budget: budget}
}

func (p *BudgetedFixtures) Name() string { return p.provider.Name() }

func (p *BudgetedFixtures) FixturesByDate(ctx context.Context, date time.Time, leagueIDs []string) ([]contracts.ProviderFixture, error) {
	if err := p.budget.Allow(ctx, p.provider.Name(), true); err != nil {
		return nil, err
	}
	return p.provider.FixturesByDate(ctx, date, leagueIDs)
}

func (p *BudgetedFixtures) FixtureResult(ctx context.Context, externalID string) (contracts.ProviderFixture, error) {
	if err := p.budget.Allow(ctx, p.provider.Name(), true); err != nil {
		return contracts.ProviderFixture{}, err
	}
	return p.provider.FixtureResult(ctx, externalID)
}

type BudgetedLiveScores struct {
	provider contracts.LiveScoreProvider
	budget   *Budget
}

func NewBudgetedLiveScores(provider contracts.LiveScoreProvider, budget *Budget) *BudgetedLiveScores {
	return &BudgetedLiveScores{provider: provider, budget: budget}
}

func (p *BudgetedLiveScores) Name() string { return p.provider.Name() }

func (p *BudgetedLiveScores) LiveScores(ctx context.Context) ([]contracts.LiveScore, error) {
	if err := p.budget.Allow(ctx, p.provider.Name(), false); err != nil {
		return nil, err
	}
	return p.provider.LiveScores(ctx)
}

var (
	_ contracts.FixtureProvider   = (*BudgetedFixtures)(nil)
	_ contracts.LiveScoreProvider = (*BudgetedLiveScores)(nil)
)
