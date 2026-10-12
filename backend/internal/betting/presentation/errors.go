package presentation

import (
	"errors"
	"net/http"

	"sportbet/internal/betting/domain"
	"sportbet/internal/shared/errs"
	"sportbet/internal/shared/httpx"
)

// writeBetError traduce los errores de dominio de una apuesta a los códigos
// del contrato (openapi.yaml) que espera el boleto del frontend. Sin esto
// salían como 500 "error interno".
func writeBetError(w http.ResponseWriter, r *http.Request, err error) {
	field := func(name, msg string) error { return errs.Validation(map[string]string{name: msg}) }
	switch {
	case errors.Is(err, domain.ErrCorrelatedSelections):
		// 422 no tiene sentinel propio en errs: se escribe con el formato común.
		httpx.WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{
			"code": "CORRELATED_SELECTIONS", "message": "Una combinada no puede tener dos selecciones del mismo partido.",
		}})
		return
	case errors.Is(err, domain.ErrMarketClosed):
		err = errs.New(errs.ErrMarketClosed, "MARKET_CLOSED", "El mercado está cerrado o el partido ya empezó.")
	case errors.Is(err, domain.ErrStakeTooLow):
		err = field("stakeCents", "El monto mínimo es 1.00.")
	case errors.Is(err, domain.ErrStakeTooHigh):
		err = field("stakeCents", "El monto máximo es 500.00.")
	case errors.Is(err, domain.ErrPayoutTooHigh):
		err = field("stakeCents", "El pago potencial supera el máximo de 10 000.00.")
	case errors.Is(err, domain.ErrTotalOddTooHigh):
		err = field("selections", "La cuota total supera el máximo de 500.00.")
	case errors.Is(err, domain.ErrInvalidSelectionCount):
		err = field("selections", "Elige entre 1 y 5 selecciones.")
	case errors.Is(err, domain.ErrSelectionNotFound):
		err = field("selections", "Alguna selección ya no existe.")
	}
	httpx.WriteError(w, r, err)
}
