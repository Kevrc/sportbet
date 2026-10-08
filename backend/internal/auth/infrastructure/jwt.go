package infrastructure

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"sportbet/internal/shared/contracts"
)

// JWTIssuer firma y valida tokens de acceso HS256 de vida corta.
type JWTIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTIssuer(secret, issuer string, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}
}

type claims struct {
	Role contracts.Role `json:"role"`
	jwt.RegisteredClaims
}

func (j *JWTIssuer) Issue(u contracts.CurrentUser) (string, time.Time, error) {
	exp := j.now().Add(j.ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(j.now()),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	s, err := tok.SignedString(j.secret)
	return s, exp, err
}

var ErrInvalidToken = errors.New("token inválido")

func (j *JWTIssuer) Parse(token string) (contracts.CurrentUser, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(j.issuer),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil || c.Subject == "" {
		return contracts.CurrentUser{}, ErrInvalidToken
	}
	return contracts.CurrentUser{ID: c.Subject, Role: c.Role}, nil
}
