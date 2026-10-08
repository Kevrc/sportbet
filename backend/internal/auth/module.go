// Package auth: registro, login, refresh token revocable y logout. Dueño de
// las tablas users y refresh_tokens. Al registrar crea la billetera con
// d.Wallet.CreateWallet dentro de la misma transacción.
//
// Dueño: Persona 1. Endpoints (ver openapi.yaml):
//   - POST /auth/register
//   - POST /auth/login
//   - POST /auth/refresh
//   - POST /auth/logout
package auth

import (
	"github.com/go-chi/chi/v5"

	"sportbet/internal/auth/infrastructure"
	"sportbet/internal/auth/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
)

// NewAuthenticator construye el middleware CurrentUser que usan todos los
// módulos. Firma congelada.
func NewAuthenticator(cfg config.Config) contracts.Authenticator {
	return presentation.NewMiddleware(infrastructure.NewJWTIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAccessTTL))
}

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	_ = r
	_ = d
}
