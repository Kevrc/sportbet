package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewRefreshToken genera un token opaco de 32 bytes aleatorios. El valor
// plano viaja al cliente en la cookie; en la base solo se guarda su hash.
func NewRefreshToken() (plain string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read nunca falla desde Go 1.24
	plain = base64.RawURLEncoding.EncodeToString(b)
	return plain, HashRefreshToken(plain)
}

func HashRefreshToken(plain string) []byte {
	h := sha256.Sum256([]byte(plain))
	return h[:]
}
