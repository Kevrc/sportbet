// Package betting: PlaceBetUseCase: valida con MarketReader, debita con WalletService e inserta la apuesta en una sola transacción; publica BetPlaced después del COMMIT.
//
// Dueño: Persona 3. Endpoints (ver openapi.yaml):
//   - POST /bets
//   - GET /bets
//   - GET /bets/{id}
package betting

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
