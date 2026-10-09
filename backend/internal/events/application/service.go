package application

import (
	"context"
	"regexp"
	"time"

	"sportbet/internal/events/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/eventbus"
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
	Markets       []Market
}

type Market struct {
	ID         string
	EventID    string
	Code       string
	Status     contracts.MarketStatus
	Selections []Selection
}

type Selection struct {
	ID      string
	Code    string
	Label   string
	OddX100 int64
	Result  contracts.SelectionOutcome
}

type Filter struct {
	Date     *time.Time
	LeagueID string
	Status   contracts.MatchStatus
	Featured *bool
	LiveOnly bool
}

type Change struct {
	Event          Event
	PreviousStatus contracts.MatchStatus
	Started        bool
	Finished       bool
	UpdatedAt      time.Time
}

type Repository interface {
	ListLeagues(context.Context) ([]League, error)
	ListEvents(context.Context, Filter) ([]Event, error)
	GetEvent(context.Context, string) (Event, error)
	SetFeatured(context.Context, string, bool) (Event, error)
	ImportFixtures(context.Context, string, time.Time, []contracts.ProviderFixture) ([]Change, error)
	ApplyLiveScores(context.Context, string, time.Time, []contracts.LiveScore) ([]Change, error)
}

type Service struct {
	repository Repository
	bus        eventbus.Bus
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func NewService(repository Repository, bus eventbus.Bus) *Service {
	return &Service{repository: repository, bus: bus}
}

func (s *Service) ListLeagues(ctx context.Context) ([]League, error) {
	return s.repository.ListLeagues(ctx)
}

func (s *Service) ListEvents(ctx context.Context, filter Filter) ([]Event, error) {
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, errs.New(errs.ErrValidation, "INVALID_EVENT_STATUS", "estado de evento inválido")
	}
	if filter.LeagueID != "" && !uuidPattern.MatchString(filter.LeagueID) {
		return nil, errs.New(errs.ErrValidation, "INVALID_LEAGUE_ID", "el ID de liga debe ser un UUID")
	}
	return s.repository.ListEvents(ctx, filter)
}

func (s *Service) GetEvent(ctx context.Context, id string) (Event, error) {
	if !uuidPattern.MatchString(id) {
		return Event{}, errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento debe ser un UUID")
	}
	return s.repository.GetEvent(ctx, id)
}

func (s *Service) SetFeatured(ctx context.Context, id string, featured bool) (Event, error) {
	if !uuidPattern.MatchString(id) {
		return Event{}, errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento debe ser un UUID")
	}
	return s.repository.SetFeatured(ctx, id, featured)
}

func (s *Service) ImportFixtures(ctx context.Context, provider string, now time.Time, fixtures []contracts.ProviderFixture) error {
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
	changes, err := s.repository.ImportFixtures(ctx, provider, now, fixtures)
	if err != nil {
		return err
	}
	s.publishChanges(ctx, changes)
	return nil
}

func (s *Service) ApplyLiveScores(ctx context.Context, provider string, now time.Time, scores []contracts.LiveScore) error {
	if provider == "" {
		return errs.New(errs.ErrValidation, "INVALID_PROVIDER", "el proveedor es obligatorio")
	}
	for _, score := range scores {
		if score.ExternalID == "" || score.HomeScore < 0 || score.AwayScore < 0 || score.Minute < 0 ||
			(score.Status != contracts.MatchLive && score.Status != contracts.MatchInterrupted) {
			return errs.New(errs.ErrValidation, "INVALID_LIVE_SCORE", "el proveedor entregó un marcador en vivo inválido")
		}
	}
	changes, err := s.repository.ApplyLiveScores(ctx, provider, now, scores)
	if err != nil {
		return err
	}
	s.publishChanges(ctx, changes)
	return nil
}

func (s *Service) publishChanges(ctx context.Context, changes []Change) {
	if s.bus == nil {
		return
	}
	for _, change := range changes {
		homeScore, awayScore := score(change.Event.HomeScore), score(change.Event.AwayScore)
		s.bus.Publish(ctx, eventbus.MatchUpdated{
			EventID: change.Event.ID, Status: change.Event.Status,
			HomeScore: homeScore, AwayScore: awayScore,
			Minute: score(change.Event.Minute), UpdatedAt: change.UpdatedAt,
		})
		if change.Started {
			s.bus.Publish(ctx, eventbus.MatchStarted{EventID: change.Event.ID, StartedAt: change.UpdatedAt})
		}
		if change.Finished {
			s.bus.Publish(ctx, eventbus.MatchFinished{
				EventID: change.Event.ID, Status: change.Event.Status,
				HomeScore: homeScore, AwayScore: awayScore, FinishedAt: change.UpdatedAt,
			})
		}
	}
}

func score(value *int) int {
	if value == nil {
		return 0
	}
	return *value
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
