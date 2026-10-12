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
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/events/application"
	"sportbet/internal/events/infrastructure"
	"sportbet/internal/events/presentation"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/contracts"
)

// Register registra las rutas, suscripciones al bus y workers del módulo.
// Usa rutas completas (r.Get("/events/{id}", ...)), nunca r.Route ni r.Mount
// sobre un prefijo que otro módulo también use: chi entra en pánico.
// resultsEvery es la frecuencia máxima de confirmación de resultados con un
// proveedor que cobra por llamada.
const resultsEvery = 10 * time.Minute

// timelinesPerRun limita las llamadas por ciclo para respetar el límite de
// 10 por minuto del plan gratuito y repartir el gasto durante el día.
const timelinesPerRun = 3

func Register(r chi.Router, d app.Deps) {
	service := application.NewService(infrastructure.NewRepository(d.DB), d.Bus)
	presentation.Register(r, service, d.Bus, d.Log, d.Auth)

	d.Lifecycle.Go("events.sync", func(ctx context.Context) error {
		if d.Config.SyncFixturesEvery <= 0 || d.Config.SyncLiveEvery <= 0 || d.Config.LiveStaleAfter <= 0 {
			return fmt.Errorf("events: los intervalos de sincronización y frescura deben ser positivos")
		}
		syncFixtures := func() {
			now := d.Clock().UTC()
			// El plan gratuito de API-Football solo permite consultar de ayer
			// a mañana: pedir pasado mañana falla y gasta una llamada.
			days := 2
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
			// La fuerza de los equipos sale de la tabla de posiciones: una
			// llamada por liga, con la misma frecuencia que el calendario.
			if standings, ok := d.Fixtures.(contracts.StandingsProvider); ok && d.Fixtures.Name() != "replay" {
				calls, updated, err := service.SyncStrengths(ctx, d.Fixtures.Name(), standings, d.Config.FeaturedLeagueIDs)
				if err != nil {
					d.Log.ErrorContext(ctx, "falló la actualización de la fuerza de los equipos", "calls", calls, "err", err)
				}
				if updated > 0 {
					d.Log.InfoContext(ctx, "fuerza de equipos actualizada desde la tabla", "calls", calls, "teams", updated)
				}
			}
		}
		syncLive := func() {
			if d.Live.Name() != "replay" {
				inPlay, err := service.HasFeaturedInPlay(ctx, d.Live.Name(), d.Clock().UTC())
				if err != nil {
					d.Log.ErrorContext(ctx, "no se pudo revisar si hay partidos destacados en juego", "err", err)
					return
				}
				if !inPlay {
					return // cero llamadas sin partidos destacados en juego
				}
			}
			scores, err := d.Live.LiveScores(ctx)
			if err != nil {
				d.Log.ErrorContext(ctx, "falló la consulta de marcadores en vivo", "err", err)
				return
			}
			if err := service.ApplyLiveScores(ctx, d.Live.Name(), d.Clock().UTC(), scores); err != nil {
				d.Log.ErrorContext(ctx, "falló la actualización de marcadores en vivo", "err", err)
			}
		}
		// Confirmar resultados cuesta llamadas: con API-Football se agrupa por
		// fecha y se hace como mucho cada 10 minutos. El replay no gasta cuota.
		var lastResults time.Time
		syncResults := func() {
			now := d.Clock().UTC()
			replay := d.Fixtures.Name() == "replay"
			cutoff := now
			if !replay {
				if !lastResults.IsZero() && now.Sub(lastResults) < resultsEvery {
					return
				}
				lastResults = now
				cutoff = now.Add(-d.Config.LiveStaleAfter)
			}
			calls, err := service.ConfirmResults(ctx, d.Fixtures.Name(), d.Fixtures, d.Config.FeaturedLeagueIDs, now, cutoff, !replay)
			if err != nil {
				d.Log.ErrorContext(ctx, "falló la confirmación de resultados", "calls", calls, "err", err)
			}
			// Línea de tiempo completa de los partidos ya terminados: una llamada
			// por partido, como mucho timelinesPerRun por ciclo.
			if calls, err := service.FillTimelines(ctx, d.Fixtures.Name(), d.Fixtures, now, timelinesPerRun); err != nil {
				d.Log.ErrorContext(ctx, "falló la carga de líneas de tiempo", "calls", calls, "err", err)
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
