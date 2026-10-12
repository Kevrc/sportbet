package application

import (
	"context"
	"regexp"
	"time"

	"sportbet/internal/markets/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type Selection struct {
	ID      string
	Code    string
	Label   string
	OddX100 int64
	Result  contracts.SelectionOutcome
}

type Market struct {
	ID         string
	EventID    string
	Code       string
	Status     contracts.MarketStatus
	Selections []Selection
}

type SelectionCode struct {
	ID   string
	Code string
}

type Repository interface {
	ListEventMarkets(context.Context, string) ([]Market, error)
	EnsureMarkets(context.Context, string, int) error
	EnsureUpcomingMarkets(context.Context, time.Time, int) error
	RepriceOpenMarkets(context.Context, time.Time, int) (int, error)
	CloseMarketsForEvent(context.Context, string) error
	CloseMarketsPastCutoff(context.Context, time.Time) error
	GetSelections(context.Context, []string) (map[string]contracts.SelectionInfo, error)
	GetEventResult(context.Context, string) (contracts.EventResult, error)
	ListFinishedSince(context.Context, time.Time) ([]contracts.EventResult, error)
	ListEventSelections(context.Context, string) ([]SelectionCode, error)
}

type Service struct {
	repository    Repository
	oneXTwoMargin int
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func NewService(repository Repository, oneXTwoMargin int) *Service {
	return &Service{repository: repository, oneXTwoMargin: oneXTwoMargin}
}

func (s *Service) ListEventMarkets(ctx context.Context, eventID string) ([]Market, error) {
	if !uuidPattern.MatchString(eventID) {
		return nil, errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento debe ser un UUID")
	}
	return s.repository.ListEventMarkets(ctx, eventID)
}

func (s *Service) EnsureMarkets(ctx context.Context, eventID string) error {
	if !uuidPattern.MatchString(eventID) {
		return errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento debe ser un UUID")
	}
	if s.oneXTwoMargin < 0 || s.oneXTwoMargin >= 100 {
		return errs.New(errs.ErrValidation, "INVALID_ODDS_MARGIN", "el margen de cuotas 1X2 debe estar entre 0 y 99")
	}
	return s.repository.EnsureMarkets(ctx, eventID, s.oneXTwoMargin)
}

func (s *Service) EnsureUpcomingMarkets(ctx context.Context, now time.Time) error {
	if s.oneXTwoMargin < 0 || s.oneXTwoMargin >= 100 {
		return errs.New(errs.ErrValidation, "INVALID_ODDS_MARGIN", "el margen de cuotas 1X2 debe estar entre 0 y 99")
	}
	return s.repository.EnsureUpcomingMarkets(ctx, now, s.oneXTwoMargin)
}

// RepriceOpenMarkets recalcula las cuotas de los mercados abiertos con la
// fuerza actual de los equipos (que se actualiza desde la tabla de
// posiciones). Las apuestas ya hechas conservan su cuota congelada; quien vea
// una cuota vieja recibe ODDS_CHANGED al apostar. Devuelve cuántas
// selecciones cambiaron.
func (s *Service) RepriceOpenMarkets(ctx context.Context, now time.Time) (int, error) {
	if s.oneXTwoMargin < 0 || s.oneXTwoMargin >= 100 {
		return 0, errs.New(errs.ErrValidation, "INVALID_ODDS_MARGIN", "el margen de cuotas 1X2 debe estar entre 0 y 99")
	}
	return s.repository.RepriceOpenMarkets(ctx, now, s.oneXTwoMargin)
}

func (s *Service) CloseMarketsForEvent(ctx context.Context, eventID string) error {
	if !uuidPattern.MatchString(eventID) {
		return errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento debe ser un UUID")
	}
	return s.repository.CloseMarketsForEvent(ctx, eventID)
}

func (s *Service) CloseMarketsPastCutoff(ctx context.Context, cutoff time.Time) error {
	return s.repository.CloseMarketsPastCutoff(ctx, cutoff)
}

func (s *Service) GetSelections(ctx context.Context, ids []string) (map[string]contracts.SelectionInfo, error) {
	if len(ids) == 0 {
		return map[string]contracts.SelectionInfo{}, nil
	}
	for _, id := range ids {
		if !uuidPattern.MatchString(id) {
			return nil, errs.New(errs.ErrValidation, "INVALID_SELECTION_ID", "el ID de selección debe ser un UUID")
		}
	}
	return s.repository.GetSelections(ctx, ids)
}

func (s *Service) GetResult(ctx context.Context, eventID string) (contracts.EventResult, error) {
	if !uuidPattern.MatchString(eventID) {
		return contracts.EventResult{}, errs.New(errs.ErrValidation, "INVALID_EVENT_ID", "el ID del evento debe ser un UUID")
	}
	return s.repository.GetEventResult(ctx, eventID)
}

func (s *Service) ResolveSelections(ctx context.Context, eventID string) (map[string]contracts.SelectionOutcome, error) {
	result, err := s.GetResult(ctx, eventID)
	if err != nil {
		return nil, err
	}
	selections, err := s.repository.ListEventSelections(ctx, eventID)
	if err != nil {
		return nil, err
	}
	outcomes := make(map[string]contracts.SelectionOutcome, len(selections))
	for _, selection := range selections {
		outcome, err := domain.ResolveSelection(selection.Code, result.Status, result.HomeScore, result.AwayScore)
		if err != nil {
			return nil, err
		}
		outcomes[selection.ID] = outcome
	}
	return outcomes, nil
}

func (s *Service) ListFinishedSince(ctx context.Context, since time.Time) ([]contracts.EventResult, error) {
	return s.repository.ListFinishedSince(ctx, since)
}

var (
	_ contracts.MarketReader = (*Service)(nil)
	_ contracts.ResultReader = (*Service)(nil)
)
