package presentation

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/httpx"
)

type usageReader interface {
	RequestsUsed(context.Context, string, time.Time) (int, error)
}

func Register(
	r chi.Router,
	cfg config.Config,
	clock func() time.Time,
	auth contracts.Authenticator,
	fixtures contracts.FixtureProvider,
	usage usageReader,
	log *slog.Logger,
) {
	h := &handler{config: cfg, clock: clock, fixtures: fixtures, usage: usage, log: log}
	admin := r.With(auth.RequireAuth, auth.RequireRole(contracts.RoleAdmin))
	admin.Get("/admin/providers/status", h.providerStatus)
	admin.Post("/admin/replay/start", h.startReplay)
}

type handler struct {
	config   config.Config
	clock    func() time.Time
	fixtures contracts.FixtureProvider
	usage    usageReader
	log      *slog.Logger
}

type statusDTO struct {
	Provider     string     `json:"provider"`
	Day          string     `json:"day"`
	RequestsUsed int        `json:"requestsUsed"`
	Budget       int        `json:"budget"`
	LastSyncAt   *time.Time `json:"lastSyncAt"`
	LastError    *string    `json:"lastError"`
}

func (h *handler) providerStatus(w http.ResponseWriter, r *http.Request) {
	provider := h.fixtures.Name()
	if provider != "replay" && provider != "apifootball" {
		provider = h.config.Provider
	}
	now := h.clock().UTC()
	requests, err := h.usage.RequestsUsed(r.Context(), provider, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, statusDTO{
		Provider: provider, Day: now.Format("2006-01-02"),
		RequestsUsed: requests, Budget: h.config.ProviderDailyBudget,
	})
}

func (h *handler) startReplay(w http.ResponseWriter, r *http.Request) {
	var request struct {
		File  string `json:"file"`
		Speed *int   `json:"speed"`
	}
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	speed := h.config.ReplaySpeed
	if request.Speed != nil {
		speed = *request.Speed
	}
	if speed <= 0 {
		httpx.WriteError(w, r, errs.New(errs.ErrValidation, "INVALID_REPLAY_SPEED", "la velocidad de replay debe ser positiva"))
		return
	}
	path, err := resolveReplayFile(h.config.ReplayFile, request.File)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	starter, ok := h.fixtures.(interface {
		StartReplay(string, int, func() time.Time) error
	})
	if !ok {
		httpx.WriteError(w, r, errs.New(errs.ErrConflict, "REPLAY_UNAVAILABLE", "no es posible iniciar un replay con el proveedor actual"))
		return
	}
	if err := starter.StartReplay(path, speed, h.clock); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
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
