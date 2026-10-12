package presentation

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sportbet/internal/betting/domain"
)

func TestWriteBetErrorMapsDomainErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
		code string
	}{
		{domain.ErrCorrelatedSelections, 422, "CORRELATED_SELECTIONS"},
		{domain.ErrMarketClosed, 422, "MARKET_CLOSED"},
		{domain.ErrStakeTooLow, 400, "VALIDATION_ERROR"},
		{domain.ErrStakeTooHigh, 400, "VALIDATION_ERROR"},
		{domain.ErrPayoutTooHigh, 400, "VALIDATION_ERROR"},
		{domain.ErrTotalOddTooHigh, 400, "VALIDATION_ERROR"},
		{domain.ErrInvalidSelectionCount, 400, "VALIDATION_ERROR"},
		{domain.ErrSelectionNotFound, 400, "VALIDATION_ERROR"},
		{fmt.Errorf("place bet: %w", domain.ErrMarketClosed), 422, "MARKET_CLOSED"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		writeBetError(rec, httptest.NewRequest(http.MethodPost, "/bets", nil), c.err)
		if rec.Code != c.want || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%v: status = %d, body = %s; want %d %s", c.err, rec.Code, rec.Body, c.want, c.code)
		}
	}
}
