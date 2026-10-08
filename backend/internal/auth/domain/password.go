package domain

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength es la longitud mínima de contraseña al registrarse.
const MinPasswordLength = 8

var ErrWeakPassword = errors.New("la contraseña debe tener al menos 8 caracteres")

func HashPassword(plain string) (string, error) {
	if len(plain) < MinPasswordLength {
		return "", ErrWeakPassword
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
