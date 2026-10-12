package presentation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/events/application"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/eventbus"
	"sportbet/internal/shared/httpx"
)

type Handler struct {
	service *application.Service
	bus     eventbus.Bus
	log     *slog.Logger
}

func Register(r chi.Router, service *application.Service, bus eventbus.Bus, log *slog.Logger, auth contracts.Authenticator) {
	h := &Handler{service: service, bus: bus, log: log}
	r.Get("/leagues", h.listLeagues)
	r.Get("/events", h.listEvents)
	r.Get("/events/live", h.listLiveEvents)
	r.Get("/events/stream", h.streamEvents)
	r.Get("/events/{id}", h.getEvent)
	admin := r.With(auth.RequireAuth, auth.RequireRole(contracts.RoleAdmin))
	admin.Patch("/admin/events/{id}", h.setFeatured)
}

type leagueDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Country  string  `json:"country"`
	Featured bool    `json:"featured"`
	LogoURL  *string `json:"logoUrl"`
	FlagURL  *string `json:"flagUrl"`
}

type matchEventDTO struct {
	Minute int     `json:"minute"`
	Extra  *int    `json:"extra"`
	Side   string  `json:"side"`
	Type   string  `json:"type"`
	Detail string  `json:"detail"`
	Player *string `json:"player"`
	Assist *string `json:"assist"`
}

type venueDTO struct {
	Name string  `json:"name"`
	City *string `json:"city"`
}

type teamDTO struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	LogoURL *string `json:"logoUrl"`
}

type eventDTO struct {
	ID            string      `json:"id"`
	League        leagueDTO   `json:"league"`
	Home          teamDTO     `json:"home"`
	Away          teamDTO     `json:"away"`
	StartAt       time.Time   `json:"startAt"`
	Status        string      `json:"status"`
	HomeScore     *int        `json:"homeScore"`
	AwayScore     *int        `json:"awayScore"`
	Minute        *int        `json:"minute"`
	Featured      bool        `json:"featured"`
	LiveUpdatedAt *time.Time  `json:"liveUpdatedAt"`
	Round         *string     `json:"round"`
	Venue         *venueDTO   `json:"venue"`
	Markets       []marketDTO `json:"markets,omitempty"`
	// Timeline solo va en el detalle y se omite si todavía no hay sucesos.
	Timeline []matchEventDTO `json:"timeline,omitempty"`
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

func (h *Handler) listLeagues(w http.ResponseWriter, r *http.Request) {
	leagues, err := h.service.ListLeagues(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]leagueDTO, 0, len(leagues))
	for _, league := range leagues {
		out = append(out, toLeague(league))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	filter, err := eventFilter(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	events, err := h.service.ListEvents(r.Context(), filter)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]eventDTO, 0, len(events))
	for _, event := range events {
		out = append(out, toEvent(event, false))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) listLiveEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context(), application.Filter{LiveOnly: true})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]eventDTO, 0, len(events))
	for _, event := range events {
		out = append(out, toEvent(event, false))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) getEvent(w http.ResponseWriter, r *http.Request) {
	event, err := h.service.GetEvent(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEvent(event, true))
}

func (h *Handler) setFeatured(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Featured *bool `json:"featured"`
	}
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if request.Featured == nil {
		httpx.WriteError(w, r, errs.New(errs.ErrValidation, "INVALID_FEATURED", "featured es obligatorio"))
		return
	}
	event, err := h.service.SetFeatured(r.Context(), chi.URLParam(r, "id"), *request.Featured)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toEvent(event, false))
}

func (h *Handler) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "el servidor no soporta streaming", http.StatusInternalServerError)
		return
	}
	events := make(chan eventbus.Event, 64)
	topics := []eventbus.Topic{
		eventbus.TopicMatchUpdated, eventbus.TopicMatchStarted, eventbus.TopicMatchFinished,
	}
	unsubscribe := make([]func(), 0, len(topics))
	for _, topic := range topics {
		unsubscribe = append(unsubscribe, h.bus.Subscribe(topic, "events.sse", func(_ context.Context, event eventbus.Event) {
			select {
			case events <- event:
			default:
				h.log.Warn("se descartó una actualización SSE por saturación del cliente")
			}
		}))
	}
	defer func() {
		for _, stop := range unsubscribe {
			stop()
		}
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	var id uint64
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event := <-events:
			data, err := json.Marshal(event)
			if err != nil {
				h.log.ErrorContext(r.Context(), "no se pudo serializar evento SSE", "err", err)
				continue
			}
			id++
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, event.Topic(), data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func eventFilter(r *http.Request) (application.Filter, error) {
	query := r.URL.Query()
	filter := application.Filter{
		LeagueID: query.Get("leagueId"),
		Status:   contracts.MatchStatus(query.Get("status")),
	}
	if value := query.Get("date"); value != "" {
		date, err := time.Parse("2006-01-02", value)
		if err != nil {
			return application.Filter{}, errs.Wrap(errs.ErrValidation, err, "INVALID_DATE", "date debe tener formato YYYY-MM-DD")
		}
		filter.Date = &date
	}
	if value := query.Get("featured"); value != "" {
		featured, err := strconv.ParseBool(value)
		if err != nil {
			return application.Filter{}, errs.Wrap(errs.ErrValidation, err, "INVALID_FEATURED", "featured debe ser true o false")
		}
		filter.Featured = &featured
	}
	return filter, nil
}

func toLeague(league application.League) leagueDTO {
	return leagueDTO{ID: league.ID, Name: league.Name, Country: league.Country, Featured: league.Featured,
		LogoURL: league.LogoURL, FlagURL: league.FlagURL}
}

func toEvent(event application.Event, detail bool) eventDTO {
	out := eventDTO{
		ID: event.ID, League: toLeague(event.League),
		Home:    teamDTO{ID: event.Home.ID, Name: event.Home.Name, LogoURL: event.Home.LogoURL},
		Away:    teamDTO{ID: event.Away.ID, Name: event.Away.Name, LogoURL: event.Away.LogoURL},
		StartAt: event.StartAt, Status: string(event.Status),
		HomeScore: event.HomeScore, AwayScore: event.AwayScore, Minute: event.Minute,
		Featured: event.Featured, LiveUpdatedAt: event.LiveUpdatedAt, Round: event.Round,
	}
	if event.VenueName != nil {
		out.Venue = &venueDTO{Name: *event.VenueName, City: event.VenueCity}
	}
	if detail {
		out.Timeline = make([]matchEventDTO, 0, len(event.Timeline))
		for _, e := range event.Timeline {
			item := matchEventDTO{Minute: e.Minute, Side: e.Side, Type: e.Type, Detail: e.Detail}
			if e.Extra > 0 {
				item.Extra = &e.Extra
			}
			if e.Player != "" {
				item.Player = &e.Player
			}
			if e.Assist != "" {
				item.Assist = &e.Assist
			}
			out.Timeline = append(out.Timeline, item)
		}
		out.Markets = make([]marketDTO, 0, len(event.Markets))
		for _, market := range event.Markets {
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
			out.Markets = append(out.Markets, item)
		}
	}
	return out
}
