package application

import (
	"context"
	"encoding/json"
	"fmt"

	"sportbet/internal/admin/domain"
	"sportbet/internal/shared/eventbus"
)

type Repository interface {
	Record(ctx context.Context, e domain.AuditEntry) error
	List(ctx context.Context, limit, offset int) ([]domain.AuditEntry, int, error)
	Stats(ctx context.Context) (domain.Stats, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// OnBetPlaced registra una apuesta nueva anunciada por betting.
func (s *Service) OnBetPlaced(ctx context.Context, e eventbus.BetPlaced) error {
	return s.record(ctx, domain.ActionBetPlaced, e.UserID, e.BetID, e)
}

// OnBetSettled registra una apuesta liquidada anunciada por settlement.
func (s *Service) OnBetSettled(ctx context.Context, e eventbus.BetSettled) error {
	return s.record(ctx, domain.ActionBetSettled, e.UserID, e.BetID, e)
}

func (s *Service) record(ctx context.Context, action, userID, betID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("admin: serializar evento: %w", err)
	}
	return s.repo.Record(ctx, domain.AuditEntry{
		ActorID: &userID, Action: action, Entity: domain.EntityBet, EntityID: &betID, Data: data,
	})
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.AuditEntry, int, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *Service) Stats(ctx context.Context) (domain.Stats, error) {
	return s.repo.Stats(ctx)
}
