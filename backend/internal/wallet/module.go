// Package wallet: billeteras y ledger (wallet_transactions). Débito atómico
// con UPDATE ... WHERE balance_cents >= monto; UNIQUE(reference, type).
//
// Dueño: Persona 3. Endpoints (ver openapi.yaml):
//   - GET /wallet
//   - GET /wallet/transactions
package wallet

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/app"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/wallet/infrastructure"
)

// NewService construye el WalletService que main.go entrega a los demás
// módulos. La firma está congelada; el dueño solo cambia el cuerpo cuando la
// implementación real con pool esté lista.
func NewService(pool *pgxpool.Pool) contracts.WalletService {
	return infrastructure.NewWalletService(pool)
}

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	_ = r
	_ = d
}
