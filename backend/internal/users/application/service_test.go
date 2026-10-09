package application_test

import (
	"context"
	"errors"
	"testing"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
	"sportbet/internal/users/application"
	"sportbet/internal/users/domain"
)

const (
	adminID = "00000000-0000-0000-0000-00000000000a"
	userID  = "00000000-0000-0000-0000-00000000000b"
)

type memUsers struct {
	byID    map[string]domain.User
	updates int
}

func (m *memUsers) FindByID(_ context.Context, id string) (domain.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return u, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "x")
	}
	return u, nil
}

func (m *memUsers) List(_ context.Context, limit, offset int) ([]domain.User, int, error) {
	var out []domain.User
	for _, u := range m.byID {
		out = append(out, u)
	}
	return out, len(out), nil
}

func (m *memUsers) Update(_ context.Context, id string, c domain.Changes) (domain.User, error) {
	m.updates++
	u, ok := m.byID[id]
	if !ok {
		return u, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "x")
	}
	if c.Role != nil {
		u.Role = *c.Role
	}
	if c.Status != nil {
		u.Status = *c.Status
	}
	m.byID[id] = u
	return u, nil
}

func setup() (*application.Service, *memUsers) {
	repo := &memUsers{byID: map[string]domain.User{
		adminID: {ID: adminID, Email: "admin@x.com", Role: contracts.RoleAdmin, Status: domain.StatusActive},
		userID:  {ID: userID, Email: "ana@x.com", Role: contracts.RoleUser, Status: domain.StatusActive},
	}}
	return application.NewService(repo), repo
}

var admin = contracts.CurrentUser{ID: adminID, Role: contracts.RoleAdmin}

func TestMe(t *testing.T) {
	svc, _ := setup()
	u, err := svc.Me(context.Background(), contracts.CurrentUser{ID: userID, Role: contracts.RoleUser})
	if err != nil || u.Email != "ana@x.com" {
		t.Fatalf("Me = %+v, %v", u, err)
	}
}

func TestUpdateBlocksUser(t *testing.T) {
	svc, _ := setup()
	blocked := domain.StatusBlocked
	u, err := svc.Update(context.Background(), admin, userID, domain.Changes{Status: &blocked})
	if err != nil || u.Status != domain.StatusBlocked {
		t.Fatalf("Update = %+v, %v", u, err)
	}
}

func TestUpdateRejections(t *testing.T) {
	user, blocked := contracts.RoleUser, domain.StatusBlocked
	cases := []struct {
		name string
		id   string
		c    domain.Changes
		want error
	}{
		{"ID inválido", "no-uuid", domain.Changes{Status: &blocked}, errs.ErrNotFound},
		{"sin cambios", userID, domain.Changes{}, errs.ErrValidation},
		{"admin se bloquea", adminID, domain.Changes{Status: &blocked}, errs.ErrConflict},
		{"admin se quita el rol", adminID, domain.Changes{Role: &user}, errs.ErrConflict},
		{"no existe", "00000000-0000-0000-0000-0000000000ff", domain.Changes{Status: &blocked}, errs.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, repo := setup()
			before := repo.updates
			_, err := svc.Update(context.Background(), admin, c.id, c.c)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v; want %v", err, c.want)
			}
			if c.want != errs.ErrNotFound && repo.updates != before {
				t.Fatal("no debió llegar al repositorio")
			}
		})
	}
}
