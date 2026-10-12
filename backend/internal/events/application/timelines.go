package application

import (
	"context"
	"errors"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

// timelineWindow limita la búsqueda a partidos recientes.
const timelineWindow = 36 * time.Hour

// FillTimelines pide la línea de tiempo completa de hasta max partidos
// terminados que todavía no la tienen: una llamada por partido, una sola vez.
// La consulta por fecha que confirma resultados no trae los sucesos, y un
// partido seguido en vivo puede haber perdido los del último tramo.
// Devuelve cuántas llamadas hizo.
func (s *Service) FillTimelines(ctx context.Context, provider string, fixtures contracts.FixtureProvider,
	now time.Time, max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	pending, err := s.repository.ListTimelinePending(ctx, provider, now.Add(-timelineWindow), max)
	if err != nil {
		return 0, err
	}
	calls := 0
	var errList []error
	for _, externalID := range pending {
		calls++
		fixture, err := fixtures.FixtureResult(ctx, externalID)
		if err != nil {
			errList = append(errList, err)
			if errors.Is(err, errs.ErrQuotaExceeded) {
				break
			}
			continue
		}
		if err := s.RefreshResult(ctx, provider, now, fixture); err != nil {
			errList = append(errList, err)
			continue
		}
		if err := s.repository.MarkTimelineFetched(ctx, provider, externalID, now); err != nil {
			errList = append(errList, err)
		}
	}
	return calls, errors.Join(errList...)
}
