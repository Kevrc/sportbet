package domain

import (
	"errors"
	"testing"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

func ptr[T any](v T) *T { return &v }

func TestChangesValidate(t *testing.T) {
	ok := []Changes{
		{Role: ptr(contracts.RoleAdmin)},
		{Status: ptr(StatusBlocked)},
		{Role: ptr(contracts.RoleUser), Status: ptr(StatusActive)},
	}
	for _, c := range ok {
		if err := c.Validate(); err != nil {
			t.Errorf("%+v: %v", c, err)
		}
	}
	bad := []Changes{
		{},
		{Role: ptr(contracts.Role("root"))},
		{Status: ptr(Status("deleted"))},
	}
	for _, c := range bad {
		if err := c.Validate(); !errors.Is(err, errs.ErrValidation) {
			t.Errorf("%+v: err = %v; want ErrValidation", c, err)
		}
	}
}

func TestCheckSelfChange(t *testing.T) {
	if err := (Changes{Status: ptr(StatusBlocked)}).CheckSelfChange("a", "b"); err != nil {
		t.Fatalf("bloquear a otro debe estar permitido: %v", err)
	}
	for _, c := range []Changes{{Status: ptr(StatusBlocked)}, {Role: ptr(contracts.RoleUser)}} {
		if err := c.CheckSelfChange("a", "a"); !errors.Is(err, errs.ErrConflict) {
			t.Errorf("%+v sobre sí mismo: err = %v; want ErrConflict", c, err)
		}
	}
	if err := (Changes{Status: ptr(StatusActive)}).CheckSelfChange("a", "a"); err != nil {
		t.Fatalf("activarse a sí mismo no es peligroso: %v", err)
	}
}

func TestIsValidID(t *testing.T) {
	if !IsValidID("0142f3e8-c542-4f66-a08f-1c9da40ab62a") {
		t.Fatal("UUID válido rechazado")
	}
	for _, s := range []string{"", "123", "0142f3e8c5424f66a08f1c9da40ab62a0000", "0142f3e8-c542-4f66-a08f-1c9da40ab62z"} {
		if IsValidID(s) {
			t.Errorf("%q no debería ser válido", s)
		}
	}
}
