package application

import (
	"context"
	"time"

	"sportbet/internal/events/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type League struct {
	ID       string
	Name     string
	Country  string
	Featured bool
}

type Team struct {
	ID   string
	Name string
}

type Event struct {
	ID            string
	League        League
	Home          Team
	Away          Team
	StartAt       time.Time
	Status        contracts.MatchStatus
	HomeScore     *int
	AwayScore     *int
	Minute        *int
	Featured      bool
	LiveUpdatedAt *time.Time
}

type Filter struct {
	Date     *time.Time
	LeagueID string
	Status   contracts.MatchStatus
	Featured *bool
	LiveOnly bool
}

type Repository interface {
	ListLeagues(context.Context) ([]League, error)
	ListEvents(context.Context, Filter) ([]Event, error)
	GetEvent(context.Context, string) (Event, error)
	SetFeatured(context.Context, string, bool) (Event, error)
	ImportFixtures(context.Context, string, []contracts.ProviderFixture) error
	ApplyLiveScores(context.Context, []contracts.LiveScore) error
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ListLeagues(ctx context.Context) ([]League, error) {
	return s.repository.ListLeagues(ctx)
}

func (s *Service) ListEvents(ctx context.Context, filter Filter) ([]Event, error) {
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, errs.New(errs.ErrValidation, "INVALID_EVENT_STATUS", "estado de evento inválido")
	}
	return s.repository.ListEvents(ctx, filter)
}

func (s *Service) GetEvent(ctx context.Context, id string) (Event, error) {
	if id == "" {
		return Event{}, errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento es obligatorio")
	}
	return s.repository.GetEvent(ctx, id)
}

func (s *Service) SetFeatured(ctx context.Context, id string, featured bool) (Event, error) {
	if id == "" {
		return Event{}, errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento es obligatorio")
	}
	return s.repository.SetFeatured(ctx, id, featured)
}

func (s *Service) ImportFixtures(ctx context.Context, provider string, fixtures []contracts.ProviderFixture) error {
	if provider == "" {
		return errs.New(errs.ErrValidation, "INVALID_PROVIDER", "el proveedor es obligatorio")
	}
	for _, fixture := range fixtures {
		err := (domain.Fixture{
			ExternalID: fixture.ExternalID,
			League:     fixture.League,
			Home:       fixture.Home,
			Away:       fixture.Away,
			StartAt:    fixture.StartAt,
			Status:     fixture.Status,
			HomeScore:  fixture.HomeScore,
			AwayScore:  fixture.AwayScore,
		}).Validate()
		if err != nil {
			return errs.Wrap(errs.ErrValidation, err, "INVALID_FIXTURE", "el proveedor entregó un evento inválido")
		}
	}
	return s.repository.ImportFixtures(ctx, provider, fixtures)
}

func (s *Service) ApplyLiveScores(ctx context.Context, scores []contracts.LiveScore) error {
	for _, score := range scores {
		if score.ExternalID == "" || score.HomeScore < 0 || score.AwayScore < 0 || score.Minute < 0 ||
			(score.Status != contracts.MatchLive && score.Status != contracts.MatchInterrupted) {
			return errs.New(errs.ErrValidation, "INVALID_LIVE_SCORE", "el proveedor entregó un marcador en vivo inválido")
		}
	}
	return s.repository.ApplyLiveScores(ctx, scores)
}

func validStatus(status contracts.MatchStatus) bool {
	switch status {
	case contracts.MatchScheduled, contracts.MatchLive, contracts.MatchInterrupted, contracts.MatchFinished,
		contracts.MatchPostponed, contracts.MatchCancelled, contracts.MatchAbandoned, contracts.MatchAwarded:
		return true
	default:
		return false
	}
}
