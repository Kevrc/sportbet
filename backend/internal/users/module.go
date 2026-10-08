// Package users: Perfil del usuario. Comparte la tabla users con auth (mismo dueño).
//
// Dueño: Persona 1. Endpoints (ver openapi.yaml):
//   - GET /me
package users

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
