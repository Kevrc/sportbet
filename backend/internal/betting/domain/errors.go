package domain

import (
	"errors"
	"sportbet/internal/shared/money"
)

var (
	ErrStakeTooLow           = errors.New("betting: stake too low")
	ErrStakeTooHigh          = errors.New("betting: stake too high")
	ErrInvalidSelectionCount = errors.New("betting: invalid selection count (1 to 5)")
	ErrPayoutTooHigh         = errors.New("betting: payout exceeds maximum allowed")
	ErrTotalOddTooHigh       = errors.New("betting: total odd exceeds maximum allowed")
	ErrCorrelatedSelections  = errors.New("betting: multiple selections from same event (CORRELATED_SELECTIONS)")
	ErrMarketClosed          = errors.New("betting: market is closed or event started")
	ErrOddsChanged           = errors.New("betting: odds changed (ODDS_CHANGED)")
	ErrSelectionNotFound     = errors.New("betting: selection not found")
)

type OddsChangedError struct {
	SelectionID string
	NewOdd      money.Odds
}

func (e *OddsChangedError) Error() string { return ErrOddsChanged.Error() }
