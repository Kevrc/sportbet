package application

import (
	"context"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/users/domain"
)

// UserRepository lee y modifica la tabla users. FindByID y Update devuelven
// errs.ErrNotFound si el usuario no existe.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (domain.User, error)
	List(ctx context.Context, limit, offset int) (users []domain.User, total int, err error)
	Update(ctx context.Context, id string, c domain.Changes) (domain.User, error)
}

type Service struct {
	users UserRepository
}

func NewService(users UserRepository) *Service { return &Service{users: users} }

var errNotFound = errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "usuario no encontrado")

// Me devuelve el perfil del usuario autenticado.
func (s *Service) Me(ctx context.Context, actor contracts.CurrentUser) (domain.User, error) {
	return s.users.FindByID(ctx, actor.ID)
}

// List devuelve una página de usuarios, del más reciente al más antiguo.
func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.User, int, error) {
	return s.users.List(ctx, limit, offset)
}

// Update cambia el rol o el estado de un usuario. Solo lo llama un admin
// (la ruta exige RequireRole(admin)).
func (s *Service) Update(ctx context.Context, actor contracts.CurrentUser, id string, c domain.Changes) (domain.User, error) {
	if !domain.IsValidID(id) {
		return domain.User{}, errNotFound
	}
	if err := c.Validate(); err != nil {
		return domain.User{}, err
	}
	if err := c.CheckSelfChange(actor.ID, id); err != nil {
		return domain.User{}, err
	}
	return s.users.Update(ctx, id, c)
}
