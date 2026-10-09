package presentation

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/httpx"
	"sportbet/internal/users/domain"
)

type userService interface {
	Me(ctx context.Context, actor contracts.CurrentUser) (domain.User, error)
	List(ctx context.Context, limit, offset int) ([]domain.User, int, error)
	Update(ctx context.Context, actor contracts.CurrentUser, id string, c domain.Changes) (domain.User, error)
}

type Handlers struct {
	svc  userService
	auth contracts.Authenticator
}

func NewHandlers(svc userService, auth contracts.Authenticator) *Handlers {
	return &Handlers{svc: svc, auth: auth}
}

// Routes registra rutas completas; /admin/* es un prefijo compartido con otros módulos.
func (h *Handlers) Routes(r chi.Router) {
	r.With(h.auth.RequireAuth).Get("/me", h.me)

	admin := r.With(h.auth.RequireAuth, h.auth.RequireRole(contracts.RoleAdmin))
	admin.Get("/admin/users", h.list)
	admin.Patch("/admin/users/{id}", h.update)
}

// ---- DTOs ----

type userDTO struct {
	ID          string         `json:"id"`
	Email       string         `json:"email"`
	DisplayName string         `json:"displayName"`
	Role        contracts.Role `json:"role"`
}

type userSummaryDTO struct {
	userDTO
	Status    domain.Status `json:"status"`
	CreatedAt time.Time     `json:"createdAt"`
}

type updateRequest struct {
	Role   *contracts.Role `json:"role"`
	Status *domain.Status  `json:"status"`
}

func toUser(u domain.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Role: u.Role}
}

func toSummary(u domain.User) userSummaryDTO {
	return userSummaryDTO{userDTO: toUser(u), Status: u.Status, CreatedAt: u.CreatedAt}
}

// ---- handlers ----

func (h *Handlers) me(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Me(r.Context(), contracts.MustCurrentUser(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toUser(u))
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	p := httpx.ParsePage(r)
	users, total, err := h.svc.List(r.Context(), p.PageSize, p.Offset())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]userSummaryDTO, 0, len(users))
	for _, u := range users {
		items = append(items, toSummary(u))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.Paged[userSummaryDTO]{
		Items: items, Page: p.Page, PageSize: p.PageSize, Total: total,
	})
}

func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	var req updateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, err := h.svc.Update(r.Context(), contracts.MustCurrentUser(r.Context()), chi.URLParam(r, "id"),
		domain.Changes{Role: req.Role, Status: req.Status})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toSummary(u))
}
