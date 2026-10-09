// Package markets: mercados, selecciones y cuotas calculadas internamente
// (Poisson con fuerza de equipo y margen; ver docs/adr/0001-decisiones-base.md).
// Cierra los mercados al recibir MatchStarted y también por reloj.
//
// Dueño: Persona 2. Endpoints (ver openapi.yaml):
//   - GET /events/{id}/markets
package markets

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/markets/application"
	"sportbet/internal/markets/infrastructure"
	"sportbet/internal/markets/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/eventbus"
)

// NewMarketReader y NewResultReader tienen la firma congelada; el dueño solo
// cambia el cuerpo cuando la implementación real esté lista.
func NewMarketReader(pool *pgxpool.Pool) contracts.MarketReader {
	return application.NewService(infrastructure.NewRepository(pool), 6)
}

func NewResultReader(pool *pgxpool.Pool) contracts.ResultReader {
	return application.NewService(infrastructure.NewRepository(pool), 6)
}

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	repository := infrastructure.NewRepository(d.DB)
	service := application.NewService(repository, d.Config.OddsMarginPercent)
	presentation.Register(r, service)

	eventbus.On(d.Bus, "markets.close-on-start", func(ctx context.Context, event eventbus.MatchStarted) {
		if err := service.CloseMarketsForEvent(ctx, event.EventID); err != nil {
			d.Log.ErrorContext(ctx, "no se pudieron cerrar los mercados al iniciar el partido",
				"event_id", event.EventID, "err", err)
		}
	})

	d.Lifecycle.Go("markets.clock", func(ctx context.Context) error {
		run := func() {
			now := d.Clock().UTC()
			if err := service.EnsureUpcomingMarkets(ctx, now); err != nil {
				d.Log.ErrorContext(ctx, "no se pudieron crear mercados para eventos próximos", "err", err)
			}
			if err := service.CloseMarketsPastCutoff(ctx, now.Add(contracts.MarketCloseLead)); err != nil {
				d.Log.ErrorContext(ctx, "no se pudieron cerrar mercados por horario", "err", err)
			}
		}
		run()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				run()
			}
		}
	})
}
