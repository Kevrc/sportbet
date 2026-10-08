// Package providers: adapters de proveedores deportivos (ReplayProvider,
// API-Football), decorator de presupuesto diario sobre provider_usage y el
// Sync Worker, único componente que consulta al proveedor.
//
// Dueño: Persona 2. Endpoints (ver openapi.yaml):
//   - GET /admin/providers/status
//   - POST /admin/replay/start
package providers

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/app"
	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/fakes"
)

// New elige el proveedor según cfg.Provider (replay | apifootball). La firma
// está congelada; el dueño solo cambia el cuerpo.
func New(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (contracts.FixtureProvider, contracts.LiveScoreProvider) {
	_, _, _ = cfg, pool, log
	return &fakes.FixtureProvider{}, &fakes.LiveScoreProvider{}
}

// Register registra las rutas y el Sync Worker (d.Lifecycle.Go).
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	_ = r
	_ = d
}
