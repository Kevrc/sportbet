package application

import (
	"context"
	"errors"
	"sync"
	"time"

	"sportbet/internal/auth/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/money"
)

type Config struct {
	InitialBalance money.Money
	RefreshTTL     time.Duration
}

// Service implementa los casos de uso de autenticación.
type Service struct {
	users   UserRepository
	refresh RefreshTokenRepository
	tokens  AccessTokenIssuer
	wallet  contracts.WalletService
	tx      TxRunner
	clock   func() time.Time
	cfg     Config
}

func NewService(users UserRepository, refresh RefreshTokenRepository, tokens AccessTokenIssuer,
	wallet contracts.WalletService, tx TxRunner, clock func() time.Time, cfg Config) *Service {
	return &Service{users: users, refresh: refresh, tokens: tokens, wallet: wallet, tx: tx, clock: clock, cfg: cfg}
}

// Session es lo que recibe el cliente al registrarse o iniciar sesión.
type Session struct {
	User             domain.User
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

// Register crea el usuario, su billetera con el saldo inicial y su primer
// refresh token en una sola transacción.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Session, error) {
	u, err := domain.NewUser(in.Email, in.DisplayName, in.Password)
	if err != nil {
		return Session{}, err
	}
	if u.PasswordHash, err = domain.HashPassword(in.Password); err != nil {
		return Session{}, err
	}

	refreshPlain, refreshHash := NewRefreshToken()
	refreshExp := s.clock().Add(s.cfg.RefreshTTL)
	err = s.tx(ctx, func(q db.Querier) error {
		created, err := s.users.Create(ctx, q, u)
		if err != nil {
			return err
		}
		u = created
		if err := s.wallet.CreateWallet(ctx, q, u.ID, s.cfg.InitialBalance); err != nil {
			return err
		}
		return s.refresh.Create(ctx, q, u.ID, refreshHash, refreshExp)
	})
	if err != nil {
		return Session{}, err
	}
	return s.session(u, refreshPlain, refreshExp)
}

var errInvalidCredentials = errs.New(errs.ErrUnauthorized, "INVALID_CREDENTIALS", "email o contraseña incorrectos")

// Login valida las credenciales y emite una sesión nueva. Responde lo mismo
// si el email no existe o si la contraseña es incorrecta.
func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	u, err := s.users.FindByEmail(ctx, domain.NormalizeEmail(email))
	if errors.Is(err, errs.ErrNotFound) {
		// Compara contra un hash ficticio para que el tiempo de respuesta no
		// revele si el email existe.
		domain.CheckPassword(dummyHash(), password)
		return Session{}, errInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if !domain.CheckPassword(u.PasswordHash, password) {
		return Session{}, errInvalidCredentials
	}
	if !u.IsActive() {
		return Session{}, errs.New(errs.ErrForbidden, "USER_BLOCKED", "la cuenta está bloqueada")
	}

	refreshPlain, refreshHash := NewRefreshToken()
	refreshExp := s.clock().Add(s.cfg.RefreshTTL)
	err = s.tx(ctx, func(q db.Querier) error {
		return s.refresh.Create(ctx, q, u.ID, refreshHash, refreshExp)
	})
	if err != nil {
		return Session{}, err
	}
	return s.session(u, refreshPlain, refreshExp)
}

var errInvalidRefresh = errs.New(errs.ErrUnauthorized, "INVALID_REFRESH_TOKEN", "sesión vencida o cerrada")

// Refresh rota el refresh token: revoca el recibido y emite uno nuevo junto
// con un token de acceso. Relee el usuario para tomar su rol y estado actuales.
func (s *Service) Refresh(ctx context.Context, refreshPlain string) (Session, error) {
	if refreshPlain == "" {
		return Session{}, errInvalidRefresh
	}
	now := s.clock()
	newPlain, newHash := NewRefreshToken()
	newExp := now.Add(s.cfg.RefreshTTL)
	var u domain.User
	err := s.tx(ctx, func(q db.Querier) error {
		userID, err := s.refresh.Consume(ctx, q, HashRefreshToken(refreshPlain), now)
		if err != nil {
			return err
		}
		if u, err = s.users.FindByID(ctx, q, userID); err != nil {
			return err
		}
		if !u.IsActive() {
			return errs.New(errs.ErrForbidden, "USER_BLOCKED", "la cuenta está bloqueada")
		}
		return s.refresh.Create(ctx, q, u.ID, newHash, newExp)
	})
	if err != nil {
		return Session{}, err
	}
	return s.session(u, newPlain, newExp)
}

// Logout revoca el refresh token. Es idempotente: cerrar una sesión ya cerrada
// o sin token no es un error.
func (s *Service) Logout(ctx context.Context, refreshPlain string) error {
	if refreshPlain == "" {
		return nil
	}
	return s.tx(ctx, func(q db.Querier) error {
		return s.refresh.Revoke(ctx, q, HashRefreshToken(refreshPlain), s.clock())
	})
}

// EnsureAdmin crea el usuario admin inicial con su billetera si el email no
// existe. Si ya existe no lo modifica (ni le sube el rol): devuelve created=false.
func (s *Service) EnsureAdmin(ctx context.Context, email, password string) (created bool, err error) {
	u, err := domain.NewUser(email, "Administrador", password)
	if err != nil {
		return false, err
	}
	if _, err := s.users.FindByEmail(ctx, u.Email); err == nil {
		return false, nil
	} else if !errors.Is(err, errs.ErrNotFound) {
		return false, err
	}
	u.Role = contracts.RoleAdmin
	if u.PasswordHash, err = domain.HashPassword(password); err != nil {
		return false, err
	}
	err = s.tx(ctx, func(q db.Querier) error {
		created, err := s.users.Create(ctx, q, u)
		if err != nil {
			return err
		}
		return s.wallet.CreateWallet(ctx, q, created.ID, s.cfg.InitialBalance)
	})
	if errors.Is(err, errs.ErrConflict) {
		return false, nil // otra instancia lo creó al mismo tiempo
	}
	return err == nil, err
}

func (s *Service) session(u domain.User, refreshPlain string, refreshExp time.Time) (Session, error) {
	access, accessExp, err := s.tokens.Issue(u.CurrentUser())
	if err != nil {
		return Session{}, err
	}
	return Session{
		User:             u,
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     refreshPlain,
		RefreshExpiresAt: refreshExp,
	}, nil
}

var dummyHash = sync.OnceValue(func() string {
	h, _ := domain.HashPassword("contraseña-ficticia-para-igualar-tiempos")
	return h
})
