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

	"sportbet/internal/auth/application"
	"sportbet/internal/auth/domain"
	"sportbet/internal/auth/presentation"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type stubService struct {
	err          error
	got          application.RegisterInput
	gotRefresh   string
	loggedOut    string
	logoutCalled bool
}

func (s *stubService) session() application.Session {
	return application.Session{
		User:             domain.User{ID: "u1", Email: "ana@mail.com", DisplayName: "Ana", Role: contracts.RoleUser, PasswordHash: "secreto"},
		AccessToken:      "jwt",
		AccessExpiresAt:  time.Now().Add(15 * time.Minute),
		RefreshToken:     "refresh-plano",
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}
}

func (s *stubService) Register(_ context.Context, in application.RegisterInput) (application.Session, error) {
	s.got = in
	return s.session(), s.err
}

func (s *stubService) Login(context.Context, string, string) (application.Session, error) {
	return s.session(), s.err
}

func (s *stubService) Refresh(_ context.Context, tok string) (application.Session, error) {
	s.gotRefresh = tok
	return s.session(), s.err
}

func (s *stubService) Logout(_ context.Context, tok string) error {
	s.logoutCalled = true
	s.loggedOut = tok
	return s.err
}

func do(t *testing.T, svc *stubService, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	presentation.NewHandlers(svc, true).Routes(r)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	r.ServeHTTP(rec, req)
	return rec
}

func TestRegisterHandler(t *testing.T) {
	svc := &stubService{}
	rec := do(t, svc, "/auth/register", `{"email":"ana@mail.com","password":"secreto123","displayName":"Ana"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	if svc.got.Email != "ana@mail.com" || svc.got.DisplayName != "Ana" {
		t.Fatalf("input = %+v", svc.got)
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["accessToken"] != "jwt" || body["user"].(map[string]any)["role"] != "user" {
		t.Fatalf("body = %v", body)
	}
	if strings.Contains(rec.Body.String(), "secreto") || strings.Contains(rec.Body.String(), "refresh-plano") {
		t.Fatal("la respuesta no debe incluir el hash ni el refresh token")
	}

	c := rec.Result().Cookies()
	if len(c) != 1 {
		t.Fatalf("cookies = %v", c)
	}
	ck := c[0]
	if ck.Name != "sb_refresh" || ck.Value != "refresh-plano" || !ck.HttpOnly || !ck.Secure ||
		ck.SameSite != http.SameSiteStrictMode || ck.Path != "/api/v1/auth" {
		t.Fatalf("cookie inesperada: %+v", ck)
	}
}

func TestLoginHandler(t *testing.T) {
	rec := do(t, &stubService{}, "/auth/login", `{"email":"ana@mail.com","password":"secreto123"}`)
	if rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 1 {
		t.Fatalf("status = %d; cookies = %v", rec.Code, rec.Result().Cookies())
	}
}

func TestAuthHandlerErrors(t *testing.T) {
	cases := []struct {
		name, path, body string
		err              error
		want             int
		code             string
	}{
		{"JSON inválido", "/auth/login", `{`, nil, 400, "INVALID_JSON"},
		{"campo desconocido", "/auth/register", `{"email":"a@b.c","admin":true}`, nil, 400, "INVALID_JSON"},
		{"credenciales", "/auth/login", `{"email":"a@b.c","password":"x"}`,
			errs.New(errs.ErrUnauthorized, "INVALID_CREDENTIALS", "x"), 401, "INVALID_CREDENTIALS"},
		{"email repetido", "/auth/register", `{"email":"a@b.c","password":"secreto123","displayName":"Ana"}`,
			errs.New(errs.ErrConflict, "EMAIL_TAKEN", "x"), 409, "EMAIL_TAKEN"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, &stubService{err: c.err}, c.path, c.body)
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), c.code) {
				t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("no debe enviar cookie en un error")
			}
		})
	}
}

func TestRefreshHandler(t *testing.T) {
	svc := &stubService{}
	rec := do(t, svc, "/auth/refresh", "", &http.Cookie{Name: "sb_refresh", Value: "viejo"})
	if rec.Code != http.StatusOK || svc.gotRefresh != "viejo" {
		t.Fatalf("status = %d; token recibido = %q", rec.Code, svc.gotRefresh)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Value != "refresh-plano" {
		t.Fatalf("debe rotar la cookie: %v", c)
	}
}

func TestRefreshHandlerFailureClearsCookie(t *testing.T) {
	svc := &stubService{err: errs.New(errs.ErrUnauthorized, "INVALID_REFRESH_TOKEN", "x")}
	rec := do(t, svc, "/auth/refresh", "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "INVALID_REFRESH_TOKEN") {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].MaxAge >= 0 || c[0].Path != "/api/v1/auth" {
		t.Fatalf("debe borrar la cookie: %+v", c)
	}
}

func TestLogoutHandler(t *testing.T) {
	svc := &stubService{}
	rec := do(t, svc, "/auth/logout", "", &http.Cookie{Name: "sb_refresh", Value: "tok"})
	if rec.Code != http.StatusNoContent || svc.loggedOut != "tok" {
		t.Fatalf("status = %d; revocado = %q", rec.Code, svc.loggedOut)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].MaxAge >= 0 {
		t.Fatalf("debe borrar la cookie: %+v", c)
	}

	svc = &stubService{}
	if rec := do(t, svc, "/auth/logout", ""); rec.Code != http.StatusNoContent || !svc.logoutCalled {
		t.Fatalf("logout sin cookie: status = %d", rec.Code)
	}
}
