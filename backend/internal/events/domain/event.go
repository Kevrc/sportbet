package domain

import (
	"errors"
	"fmt"
	"time"

	"sportbet/internal/shared/contracts"
)

var (
	ErrInvalidFixture    = errors.New("events: fixture inválido")
	ErrInvalidTransition = errors.New("events: transición de estado inválida")
)

type Fixture struct {
	ExternalID string
	League     contracts.ProviderLeague
	Home       contracts.ProviderTeam
	Away       contracts.ProviderTeam
	StartAt    time.Time
	Status     contracts.MatchStatus
	HomeScore  *int
	AwayScore  *int
}

func (f Fixture) Validate() error {
	if f.ExternalID == "" || f.League.ExternalID == "" || f.League.Name == "" ||
		f.Home.ExternalID == "" || f.Home.Name == "" ||
		f.Away.ExternalID == "" || f.Away.Name == "" ||
		f.Home.ExternalID == f.Away.ExternalID || f.StartAt.IsZero() {
		return ErrInvalidFixture
	}
	if !validStatus(f.Status) {
		return fmt.Errorf("%w: estado %q", ErrInvalidFixture, f.Status)
	}
	if (f.HomeScore == nil) != (f.AwayScore == nil) {
		return fmt.Errorf("%w: marcador incompleto", ErrInvalidFixture)
	}
	if f.HomeScore != nil && (*f.HomeScore < 0 || *f.AwayScore < 0) {
		return fmt.Errorf("%w: marcador negativo", ErrInvalidFixture)
	}
	return nil
}

func CanTransition(from, to contracts.MatchStatus, admin bool) bool {
	if !validStatus(from) || !validStatus(to) {
		return false
	}
	if from == to {
		return true
	}
	if from.IsTerminal() {
		return admin && to == contracts.MatchScheduled
	}
	switch from {
	case contracts.MatchScheduled:
		return to == contracts.MatchLive || to == contracts.MatchInterrupted || to.IsTerminal()
	case contracts.MatchLive:
		return to == contracts.MatchInterrupted || to.IsTerminal()
	case contracts.MatchInterrupted:
		return to == contracts.MatchLive || to.IsTerminal()
	default:
		return false
	}
}

func validStatus(status contracts.MatchStatus) bool {
	switch status {
	case contracts.MatchScheduled, contracts.MatchLive, contracts.MatchInterrupted,
		contracts.MatchFinished, contracts.MatchPostponed, contracts.MatchCancelled,
		contracts.MatchAbandoned, contracts.MatchAwarded:
		return true
	default:
		return false
	}
}
