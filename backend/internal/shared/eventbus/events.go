package eventbus

import (
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/money"
)

type Topic string

const (
	TopicMatchUpdated  Topic = "match.updated"  // Sync Worker → SSE
	TopicMatchStarted  Topic = "match.started"  // Sync Worker → markets (cierra mercados)
	TopicMatchFinished Topic = "match.finished" // Sync Worker → settlement
	TopicBetPlaced     Topic = "bet.placed"     // betting → admin (auditoría)
	TopicBetSettled    Topic = "bet.settled"    // settlement → admin (auditoría)
)

// Los campos llevan tags JSON porque el SSE reenvía los eventos de partido tal cual.

type MatchUpdated struct {
	EventID   string                `json:"eventId"`
	Status    contracts.MatchStatus `json:"status"`
	HomeScore int                   `json:"homeScore"`
	AwayScore int                   `json:"awayScore"`
	Minute    int                   `json:"minute"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

type MatchStarted struct {
	EventID   string    `json:"eventId"`
	StartedAt time.Time `json:"startedAt"`
}

// MatchFinished se emite para cualquier estado terminal. Status FINISHED se
// liquida con el marcador; POSTPONED, CANCELLED, ABANDONED y AWARDED anulan.
type MatchFinished struct {
	EventID    string                `json:"eventId"`
	Status     contracts.MatchStatus `json:"status"`
	HomeScore  int                   `json:"homeScore"`
	AwayScore  int                   `json:"awayScore"`
	FinishedAt time.Time             `json:"finishedAt"`
}

type BetPlaced struct {
	BetID      string      `json:"betId"`
	UserID     string      `json:"userId"`
	Stake      money.Money `json:"stakeCents"`
	TotalOdd   money.Odds  `json:"totalOddX100"`
	Selections int         `json:"selections"`
	PlacedAt   time.Time   `json:"placedAt"`
	// Legs describe cada selección para que la auditoría sea legible sin
	// consultar las tablas de betting.
	Legs []BetLeg `json:"legs,omitempty"`
}

type BetLeg struct {
	EventID    string     `json:"eventId"`
	EventName  string     `json:"eventName"`
	MarketCode string     `json:"marketCode"`
	Pick       string     `json:"pick"`
	Odd        money.Odds `json:"oddX100"`
}

type BetSettled struct {
	BetID     string              `json:"betId"`
	UserID    string              `json:"userId"`
	Status    contracts.BetStatus `json:"status"`
	Payout    money.Money         `json:"payoutCents"`
	SettledAt time.Time           `json:"settledAt"`
}

func (MatchUpdated) Topic() Topic  { return TopicMatchUpdated }
func (MatchStarted) Topic() Topic  { return TopicMatchStarted }
func (MatchFinished) Topic() Topic { return TopicMatchFinished }
func (BetPlaced) Topic() Topic     { return TopicBetPlaced }
func (BetSettled) Topic() Topic    { return TopicBetSettled }
