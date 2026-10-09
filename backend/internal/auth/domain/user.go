package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type UserStatus string

const (
	StatusActive  UserStatus = "active"
	StatusBlocked UserStatus = "blocked"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	Role         contracts.Role
	Status       UserStatus
	CreatedAt    time.Time
}

func (u User) IsActive() bool { return u.Status == StatusActive }

func (u User) CurrentUser() contracts.CurrentUser {
	return contracts.CurrentUser{ID: u.ID, Role: u.Role}
}

// NormalizeEmail recorta espacios y pasa a minúsculas; el índice único es lower(email).
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NewUser valida los datos de registro y devuelve un usuario activo con rol user.
// El hash de la contraseña se calcula aparte porque bcrypt es lento.
func NewUser(email, displayName, password string) (User, error) {
	fields := map[string]string{}
	email = NormalizeEmail(email)
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		fields["email"] = "email inválido"
	}
	displayName = strings.TrimSpace(displayName)
	if n := utf8.RuneCountInString(displayName); n < 2 || n > 40 {
		fields["displayName"] = "debe tener entre 2 y 40 caracteres"
	}
	if len(password) < MinPasswordLength {
		fields["password"] = ErrWeakPassword.Error()
	}
	if len(fields) > 0 {
		return User{}, errs.Validation(fields)
	}
	return User{
		Email:       email,
		DisplayName: displayName,
		Role:        contracts.RoleUser,
		Status:      StatusActive,
	}, nil
}
