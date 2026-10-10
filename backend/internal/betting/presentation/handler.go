package presentation

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/betting/application"
	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/httpx"
	"sportbet/internal/shared/money"
)

// BettingHandler expone la API REST para el módulo de apuestas.
type BettingHandler struct {
	useCase *application.PlaceBetUseCase
}

func NewBettingHandler(useCase *application.PlaceBetUseCase) *BettingHandler {
	return &BettingHandler{useCase: useCase}
}

// RegisterRoutes vincula los endpoints del módulo al router global.
func (h *BettingHandler) RegisterRoutes(r chi.Router, auth contracts.Authenticator) {
	r.With(auth.RequireAuth).Post("/api/v1/bets", h.PlaceBet)
	// r.With(auth.RequireAuth).Get("/api/v1/bets/{id}", h.GetBet) // TODO: Implementar consulta simple
}

type placeBetRequest struct {
	StakeCents int64 `json:"stakeCents"`
	Selections []struct {
		SelectionID string `json:"selectionId"`
		OddX100     int64  `json:"oddX100"`
	} `json:"selections"`
}

type placeBetResponse struct {
	ID                     string `json:"id"`
	TotalOddX100           int64  `json:"totalOddX100"`
	PotentialPayoutCents   int64  `json:"potentialPayoutCents"`
}

// PlaceBet procesa la solicitud del usuario para colocar una apuesta.
func (h *BettingHandler) PlaceBet(w http.ResponseWriter, r *http.Request) {
	u := contracts.MustCurrentUser(r.Context())
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		httpx.WriteError(w, r, domain.ErrInvalidSelectionCount) // En este punto falta validacion estricta
		// Mejor responder un error normal
		http.Error(w, "Idempotency-Key header is required", http.StatusBadRequest)
		return
	}

	var req placeBetRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	cmd := application.PlaceBetCommand{
		UserID:         u.ID,
		IdempotencyKey: idemKey,
		Stake:          money.FromCents(req.StakeCents),
		Selections:     make([]application.SelectionInput, len(req.Selections)),
	}

	for i, s := range req.Selections {
		cmd.Selections[i] = application.SelectionInput{
			SelectionID: s.SelectionID,
			Odd:         money.Odds(s.OddX100),
		}
	}

	res, err := h.useCase.Execute(r.Context(), cmd)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, placeBetResponse{
		ID:                   res.BetID,
		TotalOddX100:         int64(res.TotalOdd),
		PotentialPayoutCents: res.PotentialPayout.Cents(),
	})
}
