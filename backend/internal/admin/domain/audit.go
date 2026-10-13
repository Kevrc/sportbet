package domain

import (
	"encoding/json"
	"time"
)

// Acciones que registra la auditoría.
const (
	ActionBetPlaced  = "BET_PLACED"
	ActionBetSettled = "BET_SETTLED"
	EntityBet        = "bet"
)

// AuditEntry es una fila de audit_log.
type AuditEntry struct {
	ID        int64
	ActorID   *string
	ActorName *string // nombre visible del usuario, si existe
	Action    string
	Entity    string
	EntityID  *string
	Data      json.RawMessage
	// Placed es el BET_PLACED de la misma apuesta; en un BET_SETTLED dice
	// qué se había apostado.
	Placed    json.RawMessage
	CreatedAt time.Time
}

// Stats son las cifras del panel. Las de apuestas salen de la auditoría:
// cuentan desde que el módulo admin escucha el bus.
type Stats struct {
	Users       int
	BetsPlaced  int
	BetsSettled int
	StakeCents  int64
	PayoutCents int64
}
