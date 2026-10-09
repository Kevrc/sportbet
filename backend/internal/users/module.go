// Package users: perfil propio y gestión de usuarios por el admin. Lee y
// modifica la tabla users que crea auth (las dos son de la Persona 1).
//
// Dueño: Persona 1. Endpoints (ver openapi.yaml):
//   - GET /me
//   - GET /admin/users
//   - PATCH /admin/users/{id}
package users

import (
	"github.com/go-chi/chi/v5"

	"sportbet/internal/shared/app"
	"sportbet/internal/users/application"
	"sportbet/internal/users/infrastructure"
	"sportbet/internal/users/presentation"
)

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas (r.Get("/events/{id}", ...)), nunca r.Route ni r.Mount
// sobre un prefijo que otro módulo también use: chi entra en pánico.
func Register(r chi.Router, d app.Deps) {
	svc := application.NewService(infrastructure.NewUserRepository(d.DB))
	presentation.NewHandlers(svc, d.Auth).Routes(r)
}
