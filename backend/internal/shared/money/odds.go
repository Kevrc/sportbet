package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Odds es una cuota decimal en centésimas: 2.14 se guarda como 214.
// Coincide con la columna selections.odd_x100.
type Odds int64

const (
	OddsScale = 100
	// OddsOne es la cuota de una selección VOID dentro de una combinada.
	OddsOne Odds = 100
	// OddsMin es la cuota mínima válida (1.01).
	OddsMin Odds = 101
)

var ErrInvalidOdds = errors.New("money: cuota inválida")

// ParseOdds convierte "2.14" en 214.
func ParseOdds(s string) (Odds, error) {
	whole, frac, hasFrac := strings.Cut(strings.TrimSpace(s), ".")
	if whole == "" || (hasFrac && (len(frac) == 0 || len(frac) > 2)) {
		return 0, ErrInvalidOdds
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w < 0 {
		return 0, ErrInvalidOdds
	}
	var f int64
	if hasFrac {
		if len(frac) == 1 {
			frac += "0"
		}
		if f, err = strconv.ParseInt(frac, 10, 64); err != nil {
			return 0, ErrInvalidOdds
		}
	}
	return Odds(w*OddsScale + f), nil
}

func (o Odds) String() string { return fmt.Sprintf("%d.%02d", o/OddsScale, o%OddsScale) }

// Combine multiplica cuotas truncando a centésimas en cada paso.
// Combine() sin argumentos devuelve OddsOne.
func Combine(odds ...Odds) Odds {
	total := OddsOne
	for _, o := range odds {
		total = total * o / OddsScale
	}
	return total
}
