package presentation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/fakes"
	"sportbet/internal/shared/errs"
	"sportbet/internal/users/domain"
	"sportbet/internal/users/presentation"
)

type stubService struct {
	gotLimit, gotOffset int
	gotID               string
	gotChanges          domain.Changes
	err                 error
}

var ana = domain.User{ID: "u1", Email: "ana@x.com", DisplayName: "Ana", Role: contracts.RoleUser,
	Status: domain.StatusActive, CreatedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}

func (s *stubService) Me(_ context.Context, actor contracts.CurrentUser) (domain.User, error) {
	u := ana
	u.ID = actor.ID
	return u, s.err
}

func (s *stubService) List(_ context.Context, limit, offset int) ([]domain.User, int, error) {
	s.gotLimit, s.gotOffset = limit, offset
	return []domain.User{ana}, 41, s.err
}

func (s *stubService) Update(_ context.Context, _ contracts.CurrentUser, id string, c domain.Changes) (domain.User, error) {
	s.gotID, s.gotChanges = id, c
	return ana, s.err
}

func serve(svc *stubService, user *contracts.CurrentUser, method, path, body string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	presentation.NewHandlers(svc, &fakes.Authenticator{User: user}).Routes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

var (
	userActor  = &contracts.CurrentUser{ID: "u1", Role: contracts.RoleUser}
	adminActor = &contracts.CurrentUser{ID: "a1", Role: contracts.RoleAdmin}
)

func TestMe(t *testing.T) {
	rec := serve(&stubService{}, userActor, http.MethodGet, "/me", "")
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != 200 || body["id"] != "u1" || body["email"] != "ana@x.com" || body["status"] != nil {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	if rec := serve(&stubService{}, nil, http.MethodGet, "/me", ""); rec.Code != 401 {
		t.Fatalf("sin token: status = %d; want 401", rec.Code)
	}
}

func TestAdminRoutesRequireAdmin(t *testing.T) {
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/admin/users"},
		{http.MethodPatch, "/admin/users/u2"},
	} {
		if rec := serve(&stubService{}, userActor, c.method, c.path, `{}`); rec.Code != 403 {
			t.Errorf("%s %s como user: status = %d; want 403", c.method, c.path, rec.Code)
		}
	}
}

func TestListUsers(t *testing.T) {
	svc := &stubService{}
	rec := serve(svc, adminActor, http.MethodGet, "/admin/users?page=3&pageSize=10", "")
	if rec.Code != 200 || svc.gotLimit != 10 || svc.gotOffset != 20 {
		t.Fatalf("status = %d; limit = %d; offset = %d", rec.Code, svc.gotLimit, svc.gotOffset)
	}
	var body struct {
		Items []map[string]any `json:"items"`
		Page  int              `json:"page"`
		Total int              `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Page != 3 || body.Total != 41 || len(body.Items) != 1 || body.Items[0]["status"] != "active" {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestUpdateUser(t *testing.T) {
	svc := &stubService{}
	rec := serve(svc, adminActor, http.MethodPatch, "/admin/users/u2", `{"status":"blocked"}`)
	if rec.Code != 200 || svc.gotID != "u2" || svc.gotChanges.Status == nil || *svc.gotChanges.Status != "blocked" || svc.gotChanges.Role != nil {
		t.Fatalf("status = %d; id = %q; cambios = %+v", rec.Code, svc.gotID, svc.gotChanges)
	}

	if rec := serve(&stubService{}, adminActor, http.MethodPatch, "/admin/users/u2", `{"password":"x"}`); rec.Code != 400 {
		t.Fatalf("campo desconocido: status = %d; want 400", rec.Code)
	}
	conflict := &stubService{err: errs.New(errs.ErrConflict, "CANNOT_MODIFY_SELF", "x")}
	if rec := serve(conflict, adminActor, http.MethodPatch, "/admin/users/a1", `{"status":"blocked"}`); rec.Code != 409 {
		t.Fatalf("auto-bloqueo: status = %d; want 409", rec.Code)
	}
}
