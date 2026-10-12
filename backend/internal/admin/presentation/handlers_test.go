package presentation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"sportbet/internal/admin/domain"
	"sportbet/internal/admin/presentation"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/fakes"
)

type stub struct{ limit, offset int }

func (s *stub) List(_ context.Context, limit, offset int) ([]domain.AuditEntry, int, error) {
	s.limit, s.offset = limit, offset
	name, id := "Ana", "b1"
	return []domain.AuditEntry{{ID: 7, ActorName: &name, Action: domain.ActionBetPlaced, Entity: "bet", EntityID: &id,
		Data: json.RawMessage(`{"stakeCents":1000}`), CreatedAt: time.Now()}}, 31, nil
}

func (s *stub) Stats(context.Context) (domain.Stats, error) {
	return domain.Stats{Users: 4, BetsPlaced: 10, BetsSettled: 6, StakeCents: 12000, PayoutCents: 8000}, nil
}

func serve(svc *stub, role contracts.Role, path string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	presentation.NewHandlers(svc, &fakes.Authenticator{User: &contracts.CurrentUser{ID: "u", Role: role}}).Routes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestAdminEndpointsRequireAdmin(t *testing.T) {
	for _, path := range []string{"/admin/stats", "/admin/audit"} {
		if rec := serve(&stub{}, contracts.RoleUser, path); rec.Code != http.StatusForbidden {
			t.Errorf("%s como usuario: %d; want 403", path, rec.Code)
		}
	}
}

func TestStatsAndAudit(t *testing.T) {
	rec := serve(&stub{}, contracts.RoleAdmin, "/admin/stats")
	var stats map[string]int
	_ = json.Unmarshal(rec.Body.Bytes(), &stats)
	if rec.Code != 200 || stats["betsPlaced"] != 10 || stats["payoutCents"] != 8000 || stats["users"] != 4 {
		t.Fatalf("stats = %d %s", rec.Code, rec.Body)
	}

	svc := &stub{}
	rec = serve(svc, contracts.RoleAdmin, "/admin/audit?page=2&pageSize=10")
	var page struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &page)
	if rec.Code != 200 || svc.limit != 10 || svc.offset != 10 || page.Total != 31 ||
		page.Items[0]["actorName"] != "Ana" || page.Items[0]["data"].(map[string]any)["stakeCents"] != 1000.0 {
		t.Fatalf("audit = %d %s", rec.Code, rec.Body)
	}
}
