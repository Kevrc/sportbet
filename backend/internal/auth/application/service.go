package application

import (
	"context"
	"time"

	"sportbet/internal/auth/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/db"
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
