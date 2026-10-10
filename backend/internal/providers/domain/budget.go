package domain

import "errors"

var ErrInvalidDailyBudget = errors.New("providers: presupuesto diario inválido")

type RequestKind uint8

const (
	FixtureRequest RequestKind = iota
	LiveScoreRequest
)

type DailyBudget struct {
	limit   int
	reserve int
}

func NewDailyBudget(limit, reserve int) (DailyBudget, error) {
	if limit <= 0 || reserve < 0 || reserve >= limit {
		return DailyBudget{}, ErrInvalidDailyBudget
	}
	return DailyBudget{limit: limit, reserve: reserve}, nil
}

func (b DailyBudget) Limit(kind RequestKind) int {
	if kind == FixtureRequest {
		return b.limit - b.reserve
	}
	return b.limit
}

func (b DailyBudget) TotalLimit() int { return b.limit }
