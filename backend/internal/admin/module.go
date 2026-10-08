// Package admin: Auditoría y estadísticas del panel. Consume BetPlaced y BetSettled del bus.
//
// Dueño: Persona 1. Endpoints (ver openapi.yaml):
//   - GET /admin/users
//   - PATCH /admin/users/{id}
//   - GET /admin/stats
//   - GET /admin/audit
package admin

import (
	"github.com/go-chi/chi/v5"

	"sportbet/internal/shared/app"
)

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas (r.Get("/events/{id}", ...)), nunca r.Route ni r.Mount
// sobre un prefijo que otro módulo también use: chi entra en pánico.
func Register(r chi.Router, d app.Deps) {
	_ = r
	_ = d
}
