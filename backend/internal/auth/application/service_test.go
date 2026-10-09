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

type memRefresh struct {
	hashes [][]byte
}

func (m *memRefresh) Create(_ context.Context, _ db.Querier, _ string, h []byte, _ time.Time) error {
	m.hashes = append(m.hashes, h)
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
