package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name, pass string
		want       error
	}{
		{"8 ASCII", "secreto1", nil},
		{"4 caracteres de 2 bytes (8 bytes)", "ññññ", ErrWeakPassword},
		{"8 caracteres multibyte", "ññññññññ", nil},
		{"72 bytes exactos", strings.Repeat("a", 72), nil},
		{"73 bytes", strings.Repeat("a", 73), ErrPasswordTooLong},
		{"40 ñ = 80 bytes", strings.Repeat("ñ", 40), ErrPasswordTooLong},
	}
	for _, c := range cases {
		if err := ValidatePassword(c.pass); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v; want %v", c.name, err, c.want)
		}
	}
}

func TestHashPasswordTooLongIsNotInternalError(t *testing.T) {
	if _, err := HashPassword(strings.Repeat("a", 100)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("err = %v; want ErrPasswordTooLong", err)
	}
}

func TestNewUserRejectsLongPassword(t *testing.T) {
	_, err := NewUser("ana@mail.com", "Ana", strings.Repeat("a", 73))
	if err == nil || !strings.Contains(err.Error(), "VALIDATION") {
		t.Fatalf("err = %v; want validación", err)
	}
}
