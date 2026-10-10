package presentation

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/settlement/application"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/httpx"
)

type SettlementHandler struct {
	settler *application.Settler
}

func NewSettlementHandler(settler *application.Settler) *SettlementHandler {
	return &SettlementHandler{settler: settler}
}

func (h *SettlementHandler) RegisterRoutes(r chi.Router, auth contracts.Authenticator) {
	// Endpoint exclusivo para el rol 'admin'
	r.With(auth.RequireAuth, auth.RequireRole(contracts.RoleAdmin)).Post("/admin/settlement/run", h.RunSweep)
}

// RunSweep fuerza un barrido sincrónico de partidos terminados.
func (h *SettlementHandler) RunSweep(w http.ResponseWriter, r *http.Request) {
	if err := h.settler.Sweep(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
