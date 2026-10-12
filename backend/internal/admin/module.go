// Package admin: auditoría y estadísticas del panel. Consume BetPlaced y
// BetSettled del bus y los guarda en audit_log. La gestión de usuarios
// (/admin/users) vive en el módulo users.
//
// Dueño: Persona 1. Endpoints (ver openapi.yaml):
//   - GET /admin/stats
//   - GET /admin/audit
package admin

import (
	"context"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/admin/application"
	"sportbet/internal/admin/infrastructure"
	"sportbet/internal/admin/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/eventbus"
)

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas (r.Get("/events/{id}", ...)), nunca r.Route ni r.Mount
// sobre un prefijo que otro módulo también use: chi entra en pánico.
func Register(r chi.Router, d app.Deps) {
	svc := application.NewService(infrastructure.NewRepository(d.DB))
	presentation.NewHandlers(svc, d.Auth).Routes(r)

	eventbus.On(d.Bus, "admin.audit.bet-placed", func(ctx context.Context, e eventbus.BetPlaced) {
		if err := svc.OnBetPlaced(ctx, e); err != nil {
			d.Log.ErrorContext(ctx, "no se pudo auditar la apuesta", "bet_id", e.BetID, "err", err)
		}
	})
	eventbus.On(d.Bus, "admin.audit.bet-settled", func(ctx context.Context, e eventbus.BetSettled) {
		if err := svc.OnBetSettled(ctx, e); err != nil {
			d.Log.ErrorContext(ctx, "no se pudo auditar la liquidación", "bet_id", e.BetID, "err", err)
		}
	})
}
