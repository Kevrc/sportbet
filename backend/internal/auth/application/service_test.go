package application_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"sportbet/internal/auth/application"
	"sportbet/internal/auth/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/contracts/fakes"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
)

// ---- dobles de prueba ----

type memUsers struct {
	mu     sync.Mutex
	byMail map[string]domain.User
	fail   error // si no es nil, Create falla con este error
}

func (m *memUsers) Create(_ context.Context, _ db.Querier, u domain.User) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail != nil {
		return u, m.fail
	}
	if _, ok := m.byMail[u.Email]; ok {
		return u, errs.New(errs.ErrConflict, "EMAIL_TAKEN", "email ya registrado")
	}
	u.ID = "user-" + u.Email
	u.CreatedAt = time.Now()
	m.byMail[u.Email] = u
	return u, nil
}

func (m *memUsers) FindByEmail(_ context.Context, email string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byMail[email]
	if !ok {
		return u, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "no existe")
	}
	return u, nil
}

func (m *memUsers) FindByID(_ context.Context, _ db.Querier, id string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.byMail {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.User{}, errs.New(errs.ErrNotFound, "USER_NOT_FOUND", "no existe")
}

type memToken struct {
	userID  string
	exp     time.Time
	revoked bool
}

type memRefresh struct {
	hashes [][]byte
	tokens map[string]*memToken
}

func (m *memRefresh) Create(_ context.Context, _ db.Querier, userID string, h []byte, exp time.Time) error {
	m.hashes = append(m.hashes, h)
	if m.tokens == nil {
		m.tokens = map[string]*memToken{}
	}
	m.tokens[string(h)] = &memToken{userID: userID, exp: exp}
	return nil
}

func (m *memRefresh) Consume(_ context.Context, _ db.Querier, h []byte, now time.Time) (string, error) {
	tok, ok := m.tokens[string(h)]
	if !ok || tok.revoked || !tok.exp.After(now) {
		return "", errs.New(errs.ErrUnauthorized, "INVALID_REFRESH_TOKEN", "sesión vencida o cerrada")
	}
	tok.revoked = true
	return tok.userID, nil
}

func (m *memRefresh) Revoke(_ context.Context, _ db.Querier, h []byte, _ time.Time) error {
	if tok, ok := m.tokens[string(h)]; ok {
		tok.revoked = true
	}
	return nil
}

type stubIssuer struct{}

func (stubIssuer) Issue(u contracts.CurrentUser) (string, time.Time, error) {
	return "jwt-" + u.ID, time.Now().Add(15 * time.Minute), nil
}

func noTx(ctx context.Context, fn func(q db.Querier) error) error { return fn(nil) }

type env struct {
	svc     *application.Service
	users   *memUsers
	refresh *memRefresh
	wallet  *fakes.Wallet
}

func newEnv() env {
	e := env{
		users:   &memUsers{byMail: map[string]domain.User{}},
		refresh: &memRefresh{},
		wallet:  fakes.NewWallet(),
	}
	e.svc = application.NewService(e.users, e.refresh, stubIssuer{}, e.wallet, noTx, time.Now,
		application.Config{InitialBalance: 100000, RefreshTTL: time.Hour})
	return e
}

// ---- Register ----

func TestRegisterCreatesUserWalletAndSession(t *testing.T) {
	e := newEnv()
	s, err := e.svc.Register(context.Background(), application.RegisterInput{
		Email: "Ana@Mail.com", Password: "secreto123", DisplayName: "Ana",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.User.Email != "ana@mail.com" || s.AccessToken == "" || s.RefreshToken == "" {
		t.Fatalf("sesión incompleta: %+v", s)
	}
	if s.User.PasswordHash == "secreto123" || !domain.CheckPassword(s.User.PasswordHash, "secreto123") {
		t.Fatal("la contraseña no quedó hasheada")
	}
	if bal, _ := e.wallet.Balance(context.Background(), s.User.ID); bal != 100000 {
		t.Fatalf("saldo inicial = %d; want 100000", bal)
	}
	if len(e.refresh.hashes) != 1 || !bytes.Equal(e.refresh.hashes[0], application.HashRefreshToken(s.RefreshToken)) {
		t.Fatal("el refresh token guardado debe ser el hash del entregado")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	e := newEnv()
	in := application.RegisterInput{Email: "ana@mail.com", Password: "secreto123", DisplayName: "Ana"}
	if _, err := e.svc.Register(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	in.Email = "ANA@mail.com"
	if _, err := e.svc.Register(context.Background(), in); !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("err = %v; want ErrConflict", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv()
	_, err := e.svc.Register(context.Background(), application.RegisterInput{Email: "x", Password: "1", DisplayName: ""})
	if !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("err = %v; want ErrValidation", err)
	}
	if len(e.users.byMail) != 0 {
		t.Fatal("no debió crear el usuario")
	}
}

func TestRegisterPropagatesRepositoryErrors(t *testing.T) {
	e := newEnv()
	e.users.fail = errors.New("db caída")
	_, err := e.svc.Register(context.Background(), application.RegisterInput{
		Email: "ana@mail.com", Password: "secreto123", DisplayName: "Ana",
	})
	if err == nil {
		t.Fatal("esperaba error")
	}
}

// ---- Login ----

func registered(t *testing.T) env {
	t.Helper()
	e := newEnv()
	_, err := e.svc.Register(context.Background(), application.RegisterInput{
		Email: "ana@mail.com", Password: "secreto123", DisplayName: "Ana",
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLoginOK(t *testing.T) {
	e := registered(t)
	s, err := e.svc.Login(context.Background(), " ANA@mail.com ", "secreto123")
	if err != nil {
		t.Fatal(err)
	}
	if s.AccessToken == "" || s.RefreshToken == "" || len(e.refresh.hashes) != 2 {
		t.Fatalf("sesión incompleta: %+v", s)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	e := registered(t)
	for _, c := range []struct{ email, pass string }{
		{"ana@mail.com", "incorrecta"},
		{"nadie@mail.com", "secreto123"},
	} {
		_, err := e.svc.Login(context.Background(), c.email, c.pass)
		var ae *errs.Error
		if !errors.As(err, &ae) || ae.Code != "INVALID_CREDENTIALS" {
			t.Errorf("Login(%s) err = %v; want INVALID_CREDENTIALS", c.email, err)
		}
	}
}

func TestLoginBlockedUser(t *testing.T) {
	e := registered(t)
	u := e.users.byMail["ana@mail.com"]
	u.Status = domain.StatusBlocked
	e.users.byMail["ana@mail.com"] = u
	if _, err := e.svc.Login(context.Background(), "ana@mail.com", "secreto123"); !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("err = %v; want ErrForbidden", err)
	}
}

// ---- Refresh y Logout ----

func TestRefreshRotatesToken(t *testing.T) {
	e := newEnv()
	s, err := e.svc.Register(context.Background(), application.RegisterInput{
		Email: "ana@mail.com", Password: "secreto123", DisplayName: "Ana",
	})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := e.svc.Refresh(context.Background(), s.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if s2.RefreshToken == s.RefreshToken || s2.AccessToken == "" || s2.User.ID != s.User.ID {
		t.Fatalf("sesión rotada inesperada: %+v", s2)
	}
	// El token anterior ya no sirve; el nuevo sí.
	if _, err := e.svc.Refresh(context.Background(), s.RefreshToken); !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("reusar el token viejo: err = %v; want ErrUnauthorized", err)
	}
	if _, err := e.svc.Refresh(context.Background(), s2.RefreshToken); err != nil {
		t.Fatalf("el token nuevo debe servir: %v", err)
	}
}

func TestRefreshRejectsInvalidTokens(t *testing.T) {
	e := newEnv()
	for _, tok := range []string{"", "no-existe"} {
		if _, err := e.svc.Refresh(context.Background(), tok); !errors.Is(err, errs.ErrUnauthorized) {
			t.Errorf("Refresh(%q) err = %v; want ErrUnauthorized", tok, err)
		}
	}
}

func TestRefreshBlockedUser(t *testing.T) {
	e := newEnv()
	s, _ := e.svc.Register(context.Background(), application.RegisterInput{
		Email: "ana@mail.com", Password: "secreto123", DisplayName: "Ana",
	})
	u := e.users.byMail["ana@mail.com"]
	u.Status = domain.StatusBlocked
	e.users.byMail["ana@mail.com"] = u
	if _, err := e.svc.Refresh(context.Background(), s.RefreshToken); !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("err = %v; want ErrForbidden", err)
	}
}

func TestLogoutRevokesToken(t *testing.T) {
	e := newEnv()
	s, _ := e.svc.Register(context.Background(), application.RegisterInput{
		Email: "ana@mail.com", Password: "secreto123", DisplayName: "Ana",
	})
	if err := e.svc.Logout(context.Background(), s.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Logout(context.Background(), s.RefreshToken); err != nil {
		t.Fatalf("Logout debe ser idempotente: %v", err)
	}
	if err := e.svc.Logout(context.Background(), ""); err != nil {
		t.Fatalf("Logout sin token: %v", err)
	}
	if _, err := e.svc.Refresh(context.Background(), s.RefreshToken); !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("refresh tras logout: err = %v; want ErrUnauthorized", err)
	}
}

// ---- EnsureAdmin ----

func TestEnsureAdminIsIdempotent(t *testing.T) {
	e := newEnv()
	created, err := e.svc.EnsureAdmin(context.Background(), "Admin@SportBet.local", "admin-secreto")
	if err != nil || !created {
		t.Fatalf("primera vez: created = %v, %v", created, err)
	}
	u := e.users.byMail["admin@sportbet.local"]
	if u.Role != contracts.RoleAdmin || !domain.CheckPassword(u.PasswordHash, "admin-secreto") {
		t.Fatalf("admin inesperado: %+v", u)
	}
	if bal, _ := e.wallet.Balance(context.Background(), u.ID); bal != 100000 {
		t.Fatalf("saldo del admin = %d", bal)
	}
	created, err = e.svc.EnsureAdmin(context.Background(), "admin@sportbet.local", "otra-clave-123")
	if err != nil || created {
		t.Fatalf("segunda vez: created = %v, %v", created, err)
	}
	if !domain.CheckPassword(e.users.byMail["admin@sportbet.local"].PasswordHash, "admin-secreto") {
		t.Fatal("no debe cambiar la contraseña de un admin existente")
	}
}

func TestEnsureAdminDoesNotPromoteExistingUser(t *testing.T) {
	e := registered(t)
	created, err := e.svc.EnsureAdmin(context.Background(), "ana@mail.com", "admin-secreto")
	if err != nil || created || e.users.byMail["ana@mail.com"].Role != contracts.RoleUser {
		t.Fatalf("created = %v, err = %v, rol = %s", created, err, e.users.byMail["ana@mail.com"].Role)
	}
}

func TestEnsureAdminRejectsWeakPassword(t *testing.T) {
	e := newEnv()
	if _, err := e.svc.EnsureAdmin(context.Background(), "admin@x.com", "123"); !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("err = %v; want ErrValidation", err)
	}
}
