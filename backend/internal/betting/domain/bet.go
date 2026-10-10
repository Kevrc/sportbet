package domain

import (
	"sportbet/internal/shared/money"
)

type BetStatus string

const (
	StatusPending BetStatus = "PENDING"
	StatusWon     BetStatus = "WON"
	StatusLost    BetStatus = "LOST"
	StatusVoid    BetStatus = "VOID"
)

type Bet struct {
	ID                string
	UserID            string
	IdempotencyKey    string
	Stake             money.Money
	TotalOdd          money.Odds
	PotentialPayout   money.Money
	Status            BetStatus
	Payout            money.Money
	SettlementVersion int
}

type BetSelection struct {
	SelectionID string
	EventID     string
	Odd         money.Odds
	Result      string // PENDING, WON, LOST, VOID
}
