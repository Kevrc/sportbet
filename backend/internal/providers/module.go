package providers

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	providerapp "sportbet/internal/providers/application"
	"sportbet/internal/providers/infrastructure"
	"sportbet/internal/providers/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
)

// New elige el proveedor según cfg.Provider (replay | apifootball). La firma
// está congelada; el dueño solo cambia el cuerpo.
func New(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (contracts.FixtureProvider, contracts.LiveScoreProvider) {
	if log == nil {
		log = slog.Default()
	}
	var fixtures contracts.FixtureProvider
	var live contracts.LiveScoreProvider
	if cfg.Provider == "replay" {
		unavailable := infrastructure.NewUnavailableProvider(fmt.Errorf("ReplayProvider todavía no se inicializó con app.Deps.Clock"))
		fixtures, live = unavailable, unavailable
	} else {
		api, err := infrastructure.NewAPIFootball(cfg.APIFootballBaseURL, cfg.APIFootballKey, nil)
		if err != nil {
			log.Error("no se pudo configurar API-Football", "err", err)
			unavailable := infrastructure.NewUnavailableProvider(err)
			fixtures, live = unavailable, unavailable
		} else {
			fixtures, live = api, api
		}
	}
	manager := infrastructure.NewProviderManager(fixtures, live)
	budget, err := providerapp.NewBudget(
		infrastructure.NewUsageRepository(pool),
		cfg.ProviderDailyBudget,
		cfg.ProviderReserve,
		nil,
	)
	if err != nil {
		log.Error("la configuración del presupuesto del proveedor es inválida", "err", err)
		unavailable := infrastructure.NewUnavailableProvider(err)
		return unavailable, unavailable
	}
	return providerapp.NewBudgetedFixtures(manager, budget), providerapp.NewBudgetedLiveScores(manager, budget)
}

// Register registra las rutas administrativas del proveedor.
func Register(r chi.Router, d app.Deps) {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	if clocked, ok := d.Fixtures.(interface{ SetClock(func() time.Time) }); ok {
		clocked.SetClock(d.Clock)
	}
	if d.Config.Provider == "replay" {
		starter, ok := d.Fixtures.(interface {
			StartReplay(string, int, func() time.Time) error
		})
		if !ok {
			log.Error("el proveedor configurado no permite iniciar ReplayProvider")
		} else if err := starter.StartReplay(d.Config.ReplayFile, d.Config.ReplaySpeed, d.Clock); err != nil {
			log.Error("no se pudo iniciar ReplayProvider", "err", err)
		}
	}
	presentation.Register(
		r, d.Config, d.Clock, d.Auth, d.Fixtures,
		infrastructure.NewUsageRepository(d.DB), log,
	)
}
