// Package settlement: Liquida WIN/LOSS/VOID al recibir MatchFinished y con un barrido de respaldo al arrancar y cada SETTLEMENT_SWEEP_INTERVAL.
//
// Dueño: Persona 3. Endpoints (ver openapi.yaml):
//   - POST /admin/settlement/run
package settlement

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
