package infrastructure_test

import (
	"testing"
	"time"

	"sportbet/internal/auth/infrastructure"
	"sportbet/internal/shared/contracts"
)

func TestJWTIssuerUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixed }
	j := infrastructure.NewJWTIssuer("secreto-de-prueba-de-32-caracteres!!", "sportbet", 15*time.Minute, clock)

	tok, exp, err := j.Issue(contracts.CurrentUser{ID: "u1", Role: contracts.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(fixed.Add(15 * time.Minute)) {
		t.Fatalf("exp = %v; want %v", exp, fixed.Add(15*time.Minute))
	}
	// Con el mismo reloj el token es válido aunque la hora real sea otra.
	u, err := j.Parse(tok)
	if err != nil || u.ID != "u1" || u.Role != contracts.RoleAdmin {
		t.Fatalf("Parse = %+v, %v", u, err)
	}
	// Con un reloj 16 min después, el token ya venció.
	later := infrastructure.NewJWTIssuer("secreto-de-prueba-de-32-caracteres!!", "sportbet", 15*time.Minute,
		func() time.Time { return fixed.Add(16 * time.Minute) })
	if _, err := later.Parse(tok); err == nil {
		t.Fatal("el token debería estar vencido")
	}
}
