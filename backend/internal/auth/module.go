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
	"context"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/auth/application"
	"sportbet/internal/auth/infrastructure"
	"sportbet/internal/auth/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/money"
)

// NewAuthenticator construye el middleware CurrentUser que usan todos los
// módulos. Firma congelada.
func NewAuthenticator(cfg config.Config) contracts.Authenticator {
	return presentation.NewMiddleware(infrastructure.NewJWTIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAccessTTL, time.Now))
}

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	svc := application.NewService(
		infrastructure.NewUserRepository(d.DB),
		infrastructure.NewRefreshTokenRepository(),
		infrastructure.NewJWTIssuer(d.Config.JWTSecret, d.Config.JWTIssuer, d.Config.JWTAccessTTL, d.Clock),
		d.Wallet,
		func(ctx context.Context, fn func(q db.Querier) error) error { return db.WithTx(ctx, d.DB, fn) },
		d.Clock,
		application.Config{
			InitialBalance: money.FromCents(d.Config.WalletInitialBalanceCents),
			RefreshTTL:     d.Config.JWTRefreshTTL,
		},
	)
	presentation.NewHandlers(svc, d.Config.RefreshCookieSecure, d.Clock).Routes(r)

	if d.Config.AdminSeedPassword != "" {
		d.Lifecycle.Go("admin-seed", func(ctx context.Context) error {
			created, err := svc.EnsureAdmin(ctx, d.Config.AdminSeedEmail, d.Config.AdminSeedPassword)
			if err != nil {
				return err
			}
			if created {
				d.Log.Info("admin inicial creado", "email", d.Config.AdminSeedEmail)
			}
			return nil
		})
	}
}
