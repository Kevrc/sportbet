package domain

import (
	"errors"
	"testing"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

func TestNewUser(t *testing.T) {
	u, err := NewUser("  Ana@Mail.COM ", " Ana ", "secreto123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@mail.com" || u.DisplayName != "Ana" || u.Role != contracts.RoleUser || !u.IsActive() {
		t.Fatalf("usuario inesperado: %+v", u)
	}
}

func TestNewUserValidation(t *testing.T) {
	_, err := NewUser("no-es-email", "A", "corta")
	var e *errs.Error
	if !errors.As(err, &e) || !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("err = %v; want validación", err)
	}
	for _, f := range []string{"email", "displayName", "password"} {
		if e.Fields[f] == "" {
			t.Errorf("falta el error del campo %s", f)
		}
	}
}
