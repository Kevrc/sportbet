package presentation

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/markets/application"
	"sportbet/internal/shared/httpx"
)

func Register(r chi.Router, service *application.Service) {
	h := &handler{service: service}
	r.Get("/events/{id}/markets", h.listEventMarkets)
}

type handler struct {
	service *application.Service
}

type marketDTO struct {
	ID         string         `json:"id"`
	EventID    string         `json:"eventId"`
	Code       string         `json:"code"`
	Status     string         `json:"status"`
	Selections []selectionDTO `json:"selections"`
}

type selectionDTO struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Label   string `json:"label"`
	OddX100 int64  `json:"oddX100"`
	Result  string `json:"result"`
}

func (h *handler) listEventMarkets(w http.ResponseWriter, r *http.Request) {
	markets, err := h.service.ListEventMarkets(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]marketDTO, 0, len(markets))
	for _, market := range markets {
		item := marketDTO{
			ID: market.ID, EventID: market.EventID, Code: market.Code, Status: string(market.Status),
			Selections: make([]selectionDTO, 0, len(market.Selections)),
		}
		for _, selection := range market.Selections {
			item.Selections = append(item.Selections, selectionDTO{
				ID: selection.ID, Code: selection.Code, Label: selection.Label,
				OddX100: selection.OddX100, Result: string(selection.Result),
			})
		}
		out = append(out, item)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
