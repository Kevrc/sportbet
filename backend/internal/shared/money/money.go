// Package money define los tipos de dinero y cuotas. Nunca se usa float64
// para dinero ni para cuotas: Money va en centavos y Odds en centésimas.
package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Money es un monto en centavos de moneda virtual.
type Money int64

var ErrInvalidAmount = errors.New("money: monto inválido")

func FromCents(c int64) Money { return Money(c) }

// Parse convierte "12.50" o "12" en 1250 centavos. Admite hasta 2 decimales.
func Parse(s string) (Money, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" || (hasFrac && (len(frac) == 0 || len(frac) > 2)) {
		return 0, ErrInvalidAmount
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}
	var f int64
	if hasFrac {
		if len(frac) == 1 {
			frac += "0"
		}
		f, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
	}
	c := w*100 + f
	if neg {
		c = -c
	}
	return Money(c), nil
}

func (m Money) Cents() int64      { return int64(m) }
func (m Money) Add(o Money) Money { return m + o }
func (m Money) Sub(o Money) Money { return m - o }
func (m Money) IsPositive() bool  { return m > 0 }
func (m Money) IsNegative() bool  { return m < 0 }

// String devuelve el monto con 2 decimales, por ejemplo "12.50".
func (m Money) String() string {
	c := int64(m)
	sign := ""
	if c < 0 {
		sign = "-"
		c = -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// MulOdds devuelve floor(m * o / 100): el pago de un stake con una cuota.
func (m Money) MulOdds(o Odds) Money {
	return Money(int64(m) * int64(o) / OddsScale)
}
