package domain

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLength es el mínimo de caracteres (no bytes) al registrarse.
	MinPasswordLength = 8
	// MaxPasswordBytes es el límite de bcrypt: más de 72 bytes se rechaza.
	MaxPasswordBytes = 72
)

var (
	ErrWeakPassword    = errors.New("la contraseña debe tener al menos 8 caracteres")
	ErrPasswordTooLong = errors.New("la contraseña no puede superar los 72 bytes")
)

// ValidatePassword aplica el mínimo en caracteres y el máximo en bytes de bcrypt.
func ValidatePassword(plain string) error {
	if utf8.RuneCountInString(plain) < MinPasswordLength {
		return ErrWeakPassword
	}
	if len(plain) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

func HashPassword(plain string) (string, error) {
	if err := ValidatePassword(plain); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
