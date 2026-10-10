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
	if repository == nil || dailyLimit <= 0 || reserve < 0 || reserve >= dailyLimit {
		return nil, errs.New(errs.ErrValidation, "INVALID_PROVIDER_BUDGET", "la configuración del presupuesto diario es inválida")
	}
	return &Budget{repository: repository, dailyLimit: dailyLimit, reserve: reserve, clock: clock}, nil
}

func (b *Budget) Allow(ctx context.Context, provider string, reserveProtected bool) error {
	if b.clock == nil {
		return errs.New(errs.ErrValidation, "PROVIDER_CLOCK_REQUIRED", "el reloj del presupuesto diario no está configurado")
	}
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
	if b.clock == nil {
		return 0, errs.New(errs.ErrValidation, "PROVIDER_CLOCK_REQUIRED", "el reloj del presupuesto diario no está configurado")
	}
	return b.repository.RequestsUsed(ctx, provider, b.clock().UTC())
}

func (b *Budget) DailyLimit() int { return b.dailyLimit }

func (b *Budget) SetClock(clock func() time.Time) {
	if clock == nil {
		return
	}
	b.clock = clock
}

type BudgetedFixtures struct {
	provider contracts.FixtureProvider
	budget   *Budget
}

func NewBudgetedFixtures(provider contracts.FixtureProvider, budget *Budget) *BudgetedFixtures {
	return &BudgetedFixtures{provider: provider, budget: budget}
}

func (p *BudgetedFixtures) Name() string { return p.provider.Name() }

func (p *BudgetedFixtures) SetClock(clock func() time.Time) {
	p.budget.SetClock(clock)
}

func (p *BudgetedFixtures) StartReplay(path string, speed int, clock func() time.Time) error {
	starter, ok := p.provider.(interface {
		StartReplay(string, int, func() time.Time) error
	})
	if !ok {
		return errs.New(errs.ErrConflict, "REPLAY_UNAVAILABLE", "el proveedor actual no permite iniciar un replay")
	}
	return starter.StartReplay(path, speed, clock)
}

func (p *BudgetedFixtures) FixturesByDate(ctx context.Context, date time.Time, leagueIDs []string) ([]contracts.ProviderFixture, error) {
	if name := p.provider.Name(); name != "replay" && name != "unavailable" {
		if err := p.budget.Allow(ctx, p.provider.Name(), true); err != nil {
			return nil, err
		}
	}
	return p.provider.FixturesByDate(ctx, date, leagueIDs)
}

func (p *BudgetedFixtures) FixtureResult(ctx context.Context, externalID string) (contracts.ProviderFixture, error) {
	if name := p.provider.Name(); name != "replay" && name != "unavailable" {
		if err := p.budget.Allow(ctx, p.provider.Name(), true); err != nil {
			return contracts.ProviderFixture{}, err
		}
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
	if name := p.provider.Name(); name != "replay" && name != "unavailable" {
		if err := p.budget.Allow(ctx, p.provider.Name(), false); err != nil {
			return nil, err
		}
	}
	return p.provider.LiveScores(ctx)
}

var (
	_ contracts.FixtureProvider   = (*BudgetedFixtures)(nil)
	_ contracts.LiveScoreProvider = (*BudgetedLiveScores)(nil)
)
