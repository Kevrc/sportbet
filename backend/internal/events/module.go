// Package events: Ligas, equipos y eventos; SSE hacia el frontend (consume MatchUpdated, MatchStarted, MatchFinished).
//
// Dueño: Persona 2. Endpoints (ver openapi.yaml):
//   - GET /leagues
//   - GET /events
//   - GET /events/live
//   - GET /events/stream (SSE, público)
//   - GET /events/{id}
//   - PATCH /admin/events/{id}
package events

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
