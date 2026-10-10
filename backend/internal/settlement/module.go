// Package settlement: Liquida WIN/LOSS/VOID al recibir MatchFinished y con un barrido de respaldo.
package settlement

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/settlement/application"
	"sportbet/internal/settlement/infrastructure"
	"sportbet/internal/settlement/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/eventbus"
)

// Constante para el intervalo de barrido.
const SweepInterval = 5 * time.Minute

// Register registra las rutas, suscripciones al bus y workers del módulo.
func Register(r chi.Router, d app.Deps) {
	repo := infrastructure.NewRepository(d.DB)
	settler := application.NewSettler(d.DB, d.Wallet, d.Results, repo, d.Clock)

	// API Handler
	h := presentation.NewSettlementHandler(settler)
	h.RegisterRoutes(r, d.Auth)

	// EventBus: Escuchar MatchFinished
	eventbus.On(d.Bus, "settlement", func(ctx context.Context, e eventbus.MatchFinished) {
		settler.HandleMatchFinished(ctx, e)
	})

	// Worker: Barrido de respaldo en segundo plano
	d.Lifecycle.Go("settlement-sweep", func(ctx context.Context) error {
		ticker := time.NewTicker(SweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				_ = settler.Sweep(ctx)
			}
		}
	})
}
