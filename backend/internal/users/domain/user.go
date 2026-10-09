package domain

import (
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusBlocked Status = "blocked"
)

// User es la vista de perfil de un usuario: nunca incluye el hash de la contraseña.
type User struct {
	ID          string
	Email       string
	DisplayName string
	Role        contracts.Role
	Status      Status
	CreatedAt   time.Time
}

// Changes son los cambios que un admin puede aplicar a un usuario.
// Un campo nil no se modifica.
type Changes struct {
	Role   *contracts.Role
	Status *Status
}

// Validate comprueba los valores y que al menos haya un cambio.
func (c Changes) Validate() error {
	fields := map[string]string{}
	if c.Role == nil && c.Status == nil {
		fields["body"] = "indica role o status"
	}
	if c.Role != nil && *c.Role != contracts.RoleUser && *c.Role != contracts.RoleAdmin {
		fields["role"] = "debe ser user o admin"
	}
	if c.Status != nil && *c.Status != StatusActive && *c.Status != StatusBlocked {
		fields["status"] = "debe ser active o blocked"
	}
	if len(fields) > 0 {
		return errs.Validation(fields)
	}
	return nil
}

// CheckSelfChange impide que un admin se quite su propio rol o se bloquee,
// para que el sistema no quede sin administradores por error.
func (c Changes) CheckSelfChange(actorID, targetID string) error {
	if actorID != targetID {
		return nil
	}
	if (c.Role != nil && *c.Role != contracts.RoleAdmin) || (c.Status != nil && *c.Status == StatusBlocked) {
		return errs.New(errs.ErrConflict, "CANNOT_MODIFY_SELF", "no puedes quitarte el rol de admin ni bloquear tu propia cuenta")
	}
	return nil
}

// IsValidID comprueba el formato UUID antes de ir a la base.
func IsValidID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
				return false
			}
		}
	}
	return true
}
