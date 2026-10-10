// Package betting: PlaceBetUseCase: valida con MarketReader, debita con WalletService e inserta la apuesta en una sola transacción; publica BetPlaced después del COMMIT.
package betting

import (
	"github.com/go-chi/chi/v5"

	"sportbet/internal/betting/application"
	"sportbet/internal/betting/infrastructure"
	"sportbet/internal/betting/presentation"
	"sportbet/internal/shared/app"
)

// Register registra las rutas, suscripciones al bus y workers del módulo.
func Register(r chi.Router, d app.Deps) {
	repo := infrastructure.NewRepository()
	useCase := application.NewPlaceBetUseCase(d.DB, d.Markets, d.Wallet, repo, d.Clock)
	
	h := presentation.NewBettingHandler(useCase, d.DB)
	h.RegisterRoutes(r, d.Auth)
}
