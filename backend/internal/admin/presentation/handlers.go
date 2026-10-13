package presentation

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/admin/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/httpx"
)

type adminService interface {
	List(ctx context.Context, limit, offset int) ([]domain.AuditEntry, int, error)
	Stats(ctx context.Context) (domain.Stats, error)
}

type Handlers struct {
	svc  adminService
	auth contracts.Authenticator
}

func NewHandlers(svc adminService, auth contracts.Authenticator) *Handlers {
	return &Handlers{svc: svc, auth: auth}
}

// Routes registra rutas completas; /admin/* es un prefijo compartido con otros módulos.
func (h *Handlers) Routes(r chi.Router) {
	admin := r.With(h.auth.RequireAuth, h.auth.RequireRole(contracts.RoleAdmin))
	admin.Get("/admin/stats", h.stats)
	admin.Get("/admin/audit", h.audit)
}

type statsDTO struct {
	Users       int   `json:"users"`
	BetsPlaced  int   `json:"betsPlaced"`
	BetsSettled int   `json:"betsSettled"`
	StakeCents  int64 `json:"stakeCents"`
	PayoutCents int64 `json:"payoutCents"`
}

type auditDTO struct {
	ID        int64           `json:"id"`
	ActorID   *string         `json:"actorId"`
	ActorName *string         `json:"actorName"`
	Action    string          `json:"action"`
	Entity    string          `json:"entity"`
	EntityID  *string         `json:"entityId"`
	Data      json.RawMessage `json:"data"`
	Placed    json.RawMessage `json:"placed,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (h *Handlers) stats(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Stats(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, statsDTO{
		Users: s.Users, BetsPlaced: s.BetsPlaced, BetsSettled: s.BetsSettled,
		StakeCents: s.StakeCents, PayoutCents: s.PayoutCents,
	})
}

func (h *Handlers) audit(w http.ResponseWriter, r *http.Request) {
	p := httpx.ParsePage(r)
	entries, total, err := h.svc.List(r.Context(), p.PageSize, p.Offset())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]auditDTO, 0, len(entries))
	for _, e := range entries {
		data := e.Data
		if len(data) == 0 {
			data = json.RawMessage(`{}`)
		}
		items = append(items, auditDTO{
			ID: e.ID, ActorID: e.ActorID, ActorName: e.ActorName, Action: e.Action,
			Entity: e.Entity, EntityID: e.EntityID, Data: data, Placed: e.Placed, CreatedAt: e.CreatedAt,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.Paged[auditDTO]{Items: items, Page: p.Page, PageSize: p.PageSize, Total: total})
}
