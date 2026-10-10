package presentation

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/betting/application"
	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/httpx"
	"sportbet/internal/shared/money"
)

// BettingHandler expone la API REST para el módulo de apuestas.
type BettingHandler struct {
	useCase *application.PlaceBetUseCase
	pool    *pgxpool.Pool
}

func NewBettingHandler(useCase *application.PlaceBetUseCase, pool *pgxpool.Pool) *BettingHandler {
	return &BettingHandler{useCase: useCase, pool: pool}
}

// RegisterRoutes vincula los endpoints del módulo al router global.
func (h *BettingHandler) RegisterRoutes(r chi.Router, auth contracts.Authenticator) {
	r.With(auth.RequireAuth).Post("/bets", h.PlaceBet)
	r.With(auth.RequireAuth).Get("/bets", h.ListBets)
}

type placeBetRequest struct {
	StakeCents int64 `json:"stakeCents"`
	Selections []struct {
		SelectionID string `json:"selectionId"`
		OddX100     int64  `json:"oddX100"`
	} `json:"selections"`
}

type placeBetResponse struct {
	ID                   string `json:"id"`
	TotalOddX100         int64  `json:"totalOddX100"`
	PotentialPayoutCents int64  `json:"potentialPayoutCents"`
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
		if errors.Is(err, errs.ErrAlreadyProcessed) {
			bet, fetchErr := fetchBetByIdempotencyKey(r.Context(), h.pool, idemKey, u.ID)
			if fetchErr != nil {
				httpx.WriteError(w, r, fetchErr)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, bet)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}

	bet, fetchErr := fetchBetComplete(r.Context(), h.pool, res.BetID, u.ID)
	if fetchErr != nil {
		httpx.WriteError(w, r, fetchErr)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, bet)
}

// ListBets devuelve el historial de apuestas del usuario.
func (h *BettingHandler) ListBets(w http.ResponseWriter, r *http.Request) {
	u := contracts.MustCurrentUser(r.Context())
	page := httpx.ParsePage(r)

	var total int
	if err := h.pool.QueryRow(r.Context(), "SELECT count(*) FROM bets WHERE user_id = $1", u.ID).Scan(&total); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT 
			b.id, b.stake_cents, b.total_odd_x100, b.potential_payout_cents, b.status, b.payout_cents, b.placed_at,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'selectionId', bs.selection_id,
					'eventId', bs.event_id,
					'eventName', ht.name || ' vs ' || at.name,
					'marketCode', m.code,
					'selectionCode', s.code,
					'label', s.label,
					'oddX100', bs.odd_x100,
					'result', bs.result
				)) 
				FROM bet_selections bs 
				JOIN selections s ON bs.selection_id = s.id
				JOIN markets m ON s.market_id = m.id
				JOIN events e ON bs.event_id = e.id
				JOIN teams ht ON e.home_team_id = ht.id
				JOIN teams at ON e.away_team_id = at.id
				WHERE bs.bet_id = b.id), '[]'::json
			) as selections
		FROM bets b
		WHERE b.user_id = $1
		ORDER BY b.placed_at DESC
		LIMIT $2 OFFSET $3
	`, u.ID, page.PageSize, page.Offset())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]betResponse, 0)
	for rows.Next() {
		var item betResponse
		if err := rows.Scan(&item.ID, &item.StakeCents, &item.TotalOddX100, &item.PotentialPayoutCents, &item.Status, &item.PayoutCents, &item.PlacedAt, &item.Selections); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, httpx.Paged[betResponse]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	})
}
