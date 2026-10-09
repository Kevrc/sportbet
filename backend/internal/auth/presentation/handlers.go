package presentation

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/auth/application"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/httpx"
)

// RefreshCookieName y RefreshCookiePath coinciden con openapi.yaml.
const (
	RefreshCookieName = "sb_refresh"
	RefreshCookiePath = "/api/v1/auth"
)

type authService interface {
	Register(ctx context.Context, in application.RegisterInput) (application.Session, error)
	Login(ctx context.Context, email, password string) (application.Session, error)
}

type Handlers struct {
	svc          authService
	secureCookie bool
}

func NewHandlers(svc authService, secureCookie bool) *Handlers {
	return &Handlers{svc: svc, secureCookie: secureCookie}
}

// Routes registra rutas completas; nunca r.Route("/auth") porque el prefijo
// es compartido con refresh y logout.
func (h *Handlers) Routes(r chi.Router) {
	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)
}

// ---- DTOs ----

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID          string         `json:"id"`
	Email       string         `json:"email"`
	DisplayName string         `json:"displayName"`
	Role        contracts.Role `json:"role"`
}

type authResponse struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	User        userDTO   `json:"user"`
}

// ---- handlers ----

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, err := h.svc.Register(r.Context(), application.RegisterInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.writeSession(w, http.StatusCreated, s)
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.writeSession(w, http.StatusOK, s)
}

func (h *Handlers) writeSession(w http.ResponseWriter, status int, s application.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    s.RefreshToken,
		Path:     RefreshCookiePath,
		Expires:  s.RefreshExpiresAt,
		MaxAge:   int(time.Until(s.RefreshExpiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, status, authResponse{
		AccessToken: s.AccessToken,
		ExpiresAt:   s.AccessExpiresAt,
		User: userDTO{
			ID:          s.User.ID,
			Email:       s.User.Email,
			DisplayName: s.User.DisplayName,
			Role:        s.User.Role,
		},
	})
}
