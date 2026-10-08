// Package markets: mercados, selecciones y cuotas calculadas internamente
// (Poisson con fuerza de equipo y margen; ver docs/adr/0001-decisiones-base.md).
// Cierra los mercados al recibir MatchStarted y también por reloj.
//
// Dueño: Persona 2. Endpoints (ver openapi.yaml):
//   - GET /events/{id}/markets
package markets

import (
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/app"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/fakes"
)

// NewMarketReader y NewResultReader tienen la firma congelada; el dueño solo
// cambia el cuerpo cuando la implementación real esté lista.
func NewMarketReader(pool *pgxpool.Pool) contracts.MarketReader {
	_ = pool
	return fakes.NewMarketReader()
}

func NewResultReader(pool *pgxpool.Pool) contracts.ResultReader {
	_ = pool
	return fakes.NewResultReader()
}

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	_ = r
	_ = d
}
