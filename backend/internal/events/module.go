// Package events: Ligas, equipos y eventos; SSE hacia el frontend (consume MatchUpdated, MatchStarted, MatchFinished).
//
// Dueño: Persona 2. Endpoints (ver openapi.yaml):
//   - GET /leagues
//   - GET /events
//   - GET /events/live
//   - GET /events/stream (SSE, público)
//   - GET /events/{id}
//   - PATCH /admin/events/{id}
package events

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/events/application"
	"sportbet/internal/events/infrastructure"
	"sportbet/internal/events/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/errs"
)

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas (r.Get("/events/{id}", ...)), nunca r.Route ni r.Mount
// sobre un prefijo que otro módulo también use: chi entra en pánico.
func Register(r chi.Router, d app.Deps) {
	service := application.NewService(infrastructure.NewRepository(d.DB), d.Bus)
	presentation.Register(r, service, d.Bus, d.Log, d.Auth)

	d.Lifecycle.Go("events.sync", func(ctx context.Context) error {
		if d.Config.SyncFixturesEvery <= 0 || d.Config.SyncLiveEvery <= 0 || d.Config.LiveStaleAfter <= 0 {
			return fmt.Errorf("events: los intervalos de sincronización y frescura deben ser positivos")
		}
		syncFixtures := func() {
			now := d.Clock().UTC()
			days := 3
			if d.Fixtures.Name() == "replay" {
				days = 31
			}
			for offset := 0; offset < days; offset++ {
				date := now.AddDate(0, 0, offset)
				fixtures, err := d.Fixtures.FixturesByDate(ctx, date, d.Config.FeaturedLeagueIDs)
				if err != nil {
					d.Log.ErrorContext(ctx, "falló la consulta de fixtures", "date", date.Format("2006-01-02"), "err", err)
					continue
				}
				if err := service.ImportFixtures(ctx, d.Fixtures.Name(), now, fixtures); err != nil {
					d.Log.ErrorContext(ctx, "falló la importación de fixtures", "date", date.Format("2006-01-02"), "err", err)
				}
			}
		}
		syncLive := func() {
			scores, err := d.Live.LiveScores(ctx)
			if err != nil {
				d.Log.ErrorContext(ctx, "falló la consulta de marcadores en vivo", "err", err)
				return
			}
			if err := service.ApplyLiveScores(ctx, d.Live.Name(), d.Clock().UTC(), scores); err != nil {
				d.Log.ErrorContext(ctx, "falló la actualización de marcadores en vivo", "err", err)
			}
		}
		syncResults := func() {
			now := d.Clock().UTC()
			cutoff := now
			if d.Fixtures.Name() != "replay" {
				cutoff = now.Add(-d.Config.LiveStaleAfter)
			}
			candidates, err := service.ListResultCandidates(ctx, d.Fixtures.Name(), cutoff)
			if err != nil {
				d.Log.ErrorContext(ctx, "falló la búsqueda de resultados pendientes", "err", err)
				return
			}
			for _, externalID := range candidates {
				fixture, err := d.Fixtures.FixtureResult(ctx, externalID)
				if err != nil {
					d.Log.ErrorContext(ctx, "falló la consulta del resultado final", "external_id", externalID, "err", err)
					if errors.Is(err, errs.ErrQuotaExceeded) {
						return
					}
					continue
				}
				if err := service.RefreshResult(ctx, d.Fixtures.Name(), now, fixture); err != nil {
					d.Log.ErrorContext(ctx, "falló la actualización del resultado final", "external_id", externalID, "err", err)
				}
			}
		}
		syncFixtures()
		syncLive()
		syncResults()
		lastFixtureProvider := d.Fixtures.Name()
		fixturesTicker := time.NewTicker(d.Config.SyncFixturesEvery)
		defer fixturesTicker.Stop()
		liveTicker := time.NewTicker(d.Config.SyncLiveEvery)
		defer liveTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-fixturesTicker.C:
				syncFixtures()
			case <-liveTicker.C:
				if provider := d.Fixtures.Name(); provider != lastFixtureProvider {
					syncFixtures()
					lastFixtureProvider = provider
				}
				syncLive()
				syncResults()
			}
		}
	})
}
