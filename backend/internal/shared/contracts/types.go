// Package contracts define las interfaces y tipos que los módulos usan para
// hablar entre sí. Un módulo nunca importa el código interno de otro: solo
// usa estas interfaces. Mientras el dueño no entrega la implementación real,
// los demás usan los fakes de contracts/fakes.
package contracts

// MatchStatus es el estado interno de un evento deportivo.
type MatchStatus string

const (
	MatchScheduled   MatchStatus = "SCHEDULED"   // NS, TBD
	MatchLive        MatchStatus = "LIVE"        // 1H, HT, 2H, ET, BT, P, LIVE
	MatchInterrupted MatchStatus = "INTERRUPTED" // SUSP, INT
	MatchFinished    MatchStatus = "FINISHED"    // FT, AET, PEN
	MatchPostponed   MatchStatus = "POSTPONED"   // PST
	MatchCancelled   MatchStatus = "CANCELLED"   // CANC
	MatchAbandoned   MatchStatus = "ABANDONED"   // ABD
	MatchAwarded     MatchStatus = "AWARDED"     // AWD, WO
)

// IsTerminal indica si el evento ya no cambiará de estado.
func (s MatchStatus) IsTerminal() bool {
	switch s {
	case MatchFinished, MatchPostponed, MatchCancelled, MatchAbandoned, MatchAwarded:
		return true
	}
	return false
}

// VoidsSelections indica si un evento terminado en este estado anula sus selecciones.
func (s MatchStatus) VoidsSelections() bool { return s.IsTerminal() && s != MatchFinished }

type MarketStatus string

const (
	MarketOpen    MarketStatus = "OPEN"
	MarketClosed  MarketStatus = "CLOSED"
	MarketSettled MarketStatus = "SETTLED"
	MarketVoid    MarketStatus = "VOID"
)

// Códigos de mercado y de selección de la versión mínima.
const (
	MarketMatchResult = "MATCH_RESULT"        // HOME, DRAW, AWAY
	MarketTotalGoals  = "TOTAL_GOALS_2_5"     // OVER, UNDER
	MarketBTTS        = "BOTH_TEAMS_TO_SCORE" // YES, NO
)

// SelectionOutcome es el resultado de una selección o de una selección dentro de una apuesta.
type SelectionOutcome string

const (
	OutcomePending SelectionOutcome = "PENDING"
	OutcomeWon     SelectionOutcome = "WON"
	OutcomeLost    SelectionOutcome = "LOST"
	OutcomeVoid    SelectionOutcome = "VOID"
)

// BetStatus es el estado de una apuesta.
type BetStatus string

const (
	BetPending BetStatus = "PENDING"
	BetWon     BetStatus = "WON"
	BetLost    BetStatus = "LOST"
	BetVoid    BetStatus = "VOID"
)
