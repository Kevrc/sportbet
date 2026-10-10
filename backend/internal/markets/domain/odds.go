package domain

import (
	"errors"
	"math"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/money"
)

const (
	minOdds money.Odds = 101
	maxOdds money.Odds = 5000
)

var ErrInvalidStrength = errors.New("markets: fuerza de equipo fuera de rango")

type OddsSet struct {
	MatchResult [3]money.Odds
	TotalGoals  [2]money.Odds
	BothScore   [2]money.Odds
}

// CalculateOdds calcula las cuotas con distribución Poisson usando las
// fortalezas persistidas de ambos equipos; solo convierte probabilidades a
// enteros centesimales al final del cálculo.
func CalculateOdds(homeStrength, awayStrength float64, oneXTwoMarginPercent int) (OddsSet, error) {
	if math.IsNaN(homeStrength) || math.IsInf(homeStrength, 0) ||
		math.IsNaN(awayStrength) || math.IsInf(awayStrength, 0) ||
		homeStrength < 0.60 || homeStrength > 1.60 || awayStrength < 0.60 || awayStrength > 1.60 {
		return OddsSet{}, ErrInvalidStrength
	}
	if oneXTwoMarginPercent < 0 || oneXTwoMarginPercent >= 100 {
		return OddsSet{}, errors.New("markets: margen 1X2 inválido")
	}
	lambdaHome := 1.45 * homeStrength / awayStrength
	lambdaAway := 1.15 * awayStrength / homeStrength
	var homeWin, draw, awayWin, over, under, yes, no, total float64
	for home := 0; home <= 10; home++ {
		for away := 0; away <= 10; away++ {
			probability := poisson(home, lambdaHome) * poisson(away, lambdaAway)
			total += probability
			switch {
			case home > away:
				homeWin += probability
			case home == away:
				draw += probability
			default:
				awayWin += probability
			}
			if home+away >= 3 {
				over += probability
			} else {
				under += probability
			}
			if home > 0 && away > 0 {
				yes += probability
			} else {
				no += probability
			}
		}
	}

	return OddsSet{
		MatchResult: [3]money.Odds{
			toOdds(homeWin/total, oneXTwoMarginPercent),
			toOdds(draw/total, oneXTwoMarginPercent),
			toOdds(awayWin/total, oneXTwoMarginPercent),
		},
		TotalGoals: [2]money.Odds{toOdds(over/total, 5), toOdds(under/total, 5)},
		BothScore:  [2]money.Odds{toOdds(yes/total, 5), toOdds(no/total, 5)},
	}, nil
}

func poisson(goals int, lambda float64) float64 {
	probability := math.Exp(-lambda)
	for i := 1; i <= goals; i++ {
		probability *= lambda / float64(i)
	}
	return probability
}

func toOdds(probability float64, marginPercent int) money.Odds {
	odds := money.Odds(math.Floor(10000 / (probability * float64(100+marginPercent))))
	return max(minOdds, min(maxOdds, odds))
}

func ResolveSelection(code string, status contracts.MatchStatus, home, away int) (contracts.SelectionOutcome, error) {
	switch code {
	case "HOME", "DRAW", "AWAY", "OVER", "UNDER", "YES", "NO":
	default:
		return contracts.OutcomePending, errors.New("markets: código de selección desconocido")
	}
	if status.VoidsSelections() {
		return contracts.OutcomeVoid, nil
	}
	if status != contracts.MatchFinished || home < 0 || away < 0 {
		return contracts.OutcomePending, errors.New("markets: resultado aún no disponible o inválido")
	}
	switch code {
	case "HOME":
		return won(home > away), nil
	case "DRAW":
		return won(home == away), nil
	case "AWAY":
		return won(home < away), nil
	case "OVER":
		return won(home+away >= 3), nil
	case "UNDER":
		return won(home+away <= 2), nil
	case "YES":
		return won(home > 0 && away > 0), nil
	case "NO":
		return won(home == 0 || away == 0), nil
	default:
		return contracts.OutcomePending, errors.New("markets: código de selección desconocido")
	}
}

func won(matches bool) contracts.SelectionOutcome {
	if matches {
		return contracts.OutcomeWon
	}
	return contracts.OutcomeLost
}
