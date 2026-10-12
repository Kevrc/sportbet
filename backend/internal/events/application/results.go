package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

// ConfirmResults consulta el estado actual de los eventos empezados que no
// tienen un dato reciente y guarda lo que el proveedor responda.
//
// Con byDate (API-Football) hace una sola consulta por fecha, que trae todos
// los partidos del día, en vez de una por partido: con 10 partidos pendientes
// gasta 1 o 2 llamadas en lugar de 10. Las fechas anteriores a ayer (que el
// plan gratuito no permite consultar por fecha) se consultan partido a partido.
// Devuelve cuántas llamadas al proveedor hizo.
func (s *Service) ConfirmResults(ctx context.Context, provider string, fixtures contracts.FixtureProvider,
	leagueIDs []string, now, cutoff time.Time, byDate bool) (int, error) {
	candidates, err := s.ListResultCandidates(ctx, provider, cutoff)
	if err != nil || len(candidates) == 0 {
		return 0, err
	}
	calls := 0
	var errList []error
	refresh := func(f contracts.ProviderFixture) {
		if err := s.RefreshResult(ctx, provider, now, f); err != nil {
			errList = append(errList, err)
		}
	}
	one := func(externalID string) bool {
		calls++
		f, err := fixtures.FixtureResult(ctx, externalID)
		if err != nil {
			errList = append(errList, err)
			return !errors.Is(err, errs.ErrQuotaExceeded)
		}
		refresh(f)
		return true
	}

	if !byDate {
		for _, c := range candidates {
			if !one(c.ExternalID) {
				break
			}
		}
		return calls, errors.Join(errList...)
	}

	yesterday := dayUTC(now).AddDate(0, 0, -1)
	groups := map[time.Time][]string{}
	for _, c := range candidates {
		day := dayUTC(c.StartAt)
		groups[day] = append(groups[day], c.ExternalID)
	}
	days := make([]time.Time, 0, len(groups))
	for day := range groups {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	for _, day := range days {
		ids := groups[day]
		if day.Before(yesterday) {
			for _, id := range ids {
				if !one(id) {
					return calls, errors.Join(errList...)
				}
			}
			continue
		}
		calls++
		list, err := fixtures.FixturesByDate(ctx, day, leagueIDs)
		if err != nil {
			errList = append(errList, err)
			if errors.Is(err, errs.ErrQuotaExceeded) {
				break
			}
			continue
		}
		byID := make(map[string]contracts.ProviderFixture, len(list))
		for _, f := range list {
			byID[f.ExternalID] = f
		}
		for _, id := range ids {
			if f, ok := byID[id]; ok {
				refresh(f)
			}
		}
	}
	return calls, errors.Join(errList...)
}

func dayUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
