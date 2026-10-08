package presentation_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sportbet/internal/auth/infrastructure"
	"sportbet/internal/auth/presentation"
	"sportbet/internal/shared/contracts"
)

func TestMiddleware(t *testing.T) {
	issuer := infrastructure.NewJWTIssuer("secreto-de-prueba-de-32-caracteres!!", "sportbet", time.Minute)
	mw := presentation.NewMiddleware(issuer)
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := contracts.MustCurrentUser(r.Context())
		w.Write([]byte(u.ID))
	})
	admin := mw.RequireAuth(mw.RequireRole(contracts.RoleAdmin)(ok))

	userTok, _, _ := issuer.Issue(contracts.CurrentUser{ID: "u1", Role: contracts.RoleUser})
	adminTok, _, _ := issuer.Issue(contracts.CurrentUser{ID: "a1", Role: contracts.RoleAdmin})

	cases := []struct {
		name, token string
		want        int
	}{
		{"sin token", "", http.StatusUnauthorized},
		{"token basura", "xxx", http.StatusUnauthorized},
		{"usuario sin rol admin", userTok, http.StatusForbidden},
		{"admin", adminTok, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}
			rec := httptest.NewRecorder()
			admin.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("status = %d; want %d", rec.Code, c.want)
			}
		})
	}
}
