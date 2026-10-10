package presentation

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/httpx"
)

type WalletHandler struct {
	service contracts.WalletService
	pool    *pgxpool.Pool
}

func NewWalletHandler(service contracts.WalletService, pool *pgxpool.Pool) *WalletHandler {
	return &WalletHandler{service: service, pool: pool}
}

func (h *WalletHandler) RegisterRoutes(r chi.Router, auth contracts.Authenticator) {
	r.With(auth.RequireAuth).Get("/wallet", h.GetWallet)
	r.With(auth.RequireAuth).Get("/wallet/transactions", h.ListTransactions)
}

// GetWallet devuelve el saldo actual del usuario.
func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	u := contracts.MustCurrentUser(r.Context())

	bal, err := h.service.Balance(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"balanceCents": bal.Cents(),
		"currency":     "VIRT",
	})
}

// ListTransactions devuelve el historial de movimientos (paginado).
func (h *WalletHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	u := contracts.MustCurrentUser(r.Context())
	page := httpx.ParsePage(r)

	// Consulta directa a pgxpool porque WalletService no incluye este método.
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, type, amount_cents, balance_after_cents, reference, created_at
		FROM wallet_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, u.ID, page.PageSize, page.Offset())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	type tx struct {
		ID                string      `json:"id"`
		Type              string      `json:"type"`
		AmountCents       int64       `json:"amountCents"`
		BalanceAfterCents int64       `json:"balanceAfterCents"`
		Reference         string      `json:"reference"`
		CreatedAt         interface{} `json:"createdAt"`
	}
	items := make([]tx, 0)
	for rows.Next() {
		var item tx
		if err := rows.Scan(&item.ID, &item.Type, &item.AmountCents, &item.BalanceAfterCents, &item.Reference, &item.CreatedAt); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		items = append(items, item)
	}

	var total int
	h.pool.QueryRow(r.Context(), "SELECT count(*) FROM wallet_transactions WHERE user_id = $1", u.ID).Scan(&total)

	httpx.WriteJSON(w, http.StatusOK, httpx.Paged[tx]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	})
}
