// Package providers: adapters de proveedores deportivos (ReplayProvider,
// API-Football), decorator de presupuesto diario sobre provider_usage y el
// Sync Worker, único componente que consulta al proveedor.
//
// Dueño: Persona 2. Endpoints (ver openapi.yaml):
//   - GET /admin/providers/status
//   - POST /admin/replay/start
package providers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	providerapp "sportbet/internal/providers/application"
	"sportbet/internal/providers/infrastructure"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/httpx"
)

// New elige el proveedor según cfg.Provider (replay | apifootball). La firma
// está congelada; el dueño solo cambia el cuerpo.
func New(cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (contracts.FixtureProvider, contracts.LiveScoreProvider) {
	if log == nil {
		log = slog.Default()
	}
	var fixtures contracts.FixtureProvider
	var live contracts.LiveScoreProvider
	if cfg.Provider == "replay" {
		unavailable := infrastructure.NewUnavailableProvider(fmt.Errorf("ReplayProvider todavía no se inicializó con app.Deps.Clock"))
		fixtures, live = unavailable, unavailable
	} else {
		api, err := infrastructure.NewAPIFootball(cfg.APIFootballBaseURL, cfg.APIFootballKey, nil)
		if err != nil {
			log.Error("no se pudo configurar API-Football", "err", err)
			unavailable := infrastructure.NewUnavailableProvider(err)
			fixtures, live = unavailable, unavailable
		} else {
			fixtures, live = api, api
		}
	}
	manager := infrastructure.NewProviderManager(fixtures, live)
	budget, err := providerapp.NewBudget(
		infrastructure.NewUsageRepository(pool),
		cfg.ProviderDailyBudget,
		cfg.ProviderReserve,
		nil,
	)
	if err != nil {
		log.Error("la configuración del presupuesto del proveedor es inválida", "err", err)
		unavailable := infrastructure.NewUnavailableProvider(err)
		return unavailable, unavailable
	}
	return providerapp.NewBudgetedFixtures(manager, budget), providerapp.NewBudgetedLiveScores(manager, budget)
}

// Register registra las rutas y el Sync Worker (d.Lifecycle.Go).
// Usa rutas completas, nunca r.Route ni r.Mount sobre un prefijo compartido.
func Register(r chi.Router, d app.Deps) {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	if clocked, ok := d.Fixtures.(interface{ SetClock(func() time.Time) }); ok {
		clocked.SetClock(d.Clock)
	}
	if d.Config.Provider == "replay" {
		starter, ok := d.Fixtures.(interface {
			StartReplay(string, int, func() time.Time) error
		})
		if !ok {
			log.Error("el proveedor configurado no permite iniciar ReplayProvider")
		} else if err := starter.StartReplay(d.Config.ReplayFile, d.Config.ReplaySpeed, d.Clock); err != nil {
			log.Error("no se pudo iniciar ReplayProvider", "err", err)
		}
	}
	usage := infrastructure.NewUsageRepository(d.DB)
	admin := r.With(d.Auth.RequireAuth, d.Auth.RequireRole(contracts.RoleAdmin))
	admin.Get("/admin/providers/status", func(w http.ResponseWriter, r *http.Request) {
		provider := d.Fixtures.Name()
		if provider != "replay" && provider != "apifootball" {
			provider = d.Config.Provider
		}
		now := d.Clock().UTC()
		requests, err := usage.RequestsUsed(r.Context(), provider, now)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, providerStatus{
			Provider: provider, Day: now.Format("2006-01-02"),
			RequestsUsed: requests, Budget: d.Config.ProviderDailyBudget,
		})
	})
	admin.Post("/admin/replay/start", func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			File  string `json:"file"`
			Speed *int   `json:"speed"`
		}
		if err := httpx.DecodeJSON(r, &request); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		speed := d.Config.ReplaySpeed
		if request.Speed != nil {
			speed = *request.Speed
		}
		if speed <= 0 {
			httpx.WriteError(w, r, errs.New(errs.ErrValidation, "INVALID_REPLAY_SPEED", "la velocidad de replay debe ser positiva"))
			return
		}
		path, err := resolveReplayFile(d.Config.ReplayFile, request.File)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		starter, ok := d.Fixtures.(interface {
			StartReplay(string, int, func() time.Time) error
		})
		if !ok {
			httpx.WriteError(w, r, errs.New(errs.ErrConflict, "REPLAY_UNAVAILABLE", "no es posible iniciar un replay con el proveedor actual"))
			return
		}
		if err := starter.StartReplay(path, speed, d.Clock); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

type providerStatus struct {
	Provider     string     `json:"provider"`
	Day          string     `json:"day"`
	RequestsUsed int        `json:"requestsUsed"`
	Budget       int        `json:"budget"`
	LastSyncAt   *time.Time `json:"lastSyncAt"`
	LastError    *string    `json:"lastError"`
}

func resolveReplayFile(configured, requested string) (string, error) {
	root, err := filepath.Abs(filepath.Dir(configured))
	if err != nil {
		return "", errs.Wrap(errs.ErrValidation, err, "INVALID_REPLAY_FILE", "no se pudo resolver el directorio de replay")
	}
	path := configured
	if requested != "" {
		if filepath.IsAbs(requested) || filepath.Base(requested) != requested || requested == "." || requested == ".." {
			return "", errs.New(errs.ErrValidation, "INVALID_REPLAY_FILE", "solo se permite un nombre de archivo del directorio de replay")
		}
		path = filepath.Join(root, requested)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errs.Wrap(errs.ErrValidation, err, "INVALID_REPLAY_FILE", "el directorio de replay no existe")
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", errs.Wrap(errs.ErrValidation, err, "INVALID_REPLAY_FILE", "el archivo de replay no existe")
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errs.New(errs.ErrValidation, "INVALID_REPLAY_FILE", "el archivo de replay debe permanecer dentro del directorio permitido")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", errs.Wrap(errs.ErrValidation, err, "INVALID_REPLAY_FILE", "no se pudo leer el archivo de replay")
	}
	if !info.Mode().IsRegular() {
		return "", errs.New(errs.ErrValidation, "INVALID_REPLAY_FILE", "el replay debe ser un archivo regular")
	}
	return resolvedPath, nil
}
