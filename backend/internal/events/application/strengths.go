package application

import (
	"context"
	"errors"
	"slices"

	"sportbet/internal/events/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

// maxStandingsLeagues limita las llamadas: una por liga.
const maxStandingsLeagues = 5

// StandingsLeague es una liga importada con su temporada en curso.
type StandingsLeague struct {
	ExternalID string
	Season     int
}

// SyncStrengths consulta la tabla de posiciones de cada liga (una llamada por
// liga) y actualiza la fuerza de sus equipos, que el modelo de cuotas usa
// para que un favorito no pague lo mismo que un equipo débil. Si allowed no
// está vacío, solo consulta esas ligas. Devuelve llamadas hechas y equipos
// actualizados.
func (s *Service) SyncStrengths(ctx context.Context, provider string, standings contracts.StandingsProvider,
	allowed []string) (calls, updated int, err error) {
	leagues, err := s.repository.ListStandingsLeagues(ctx, provider)
	if err != nil {
		return 0, 0, err
	}
	var errList []error
	for _, league := range leagues {
		if calls >= maxStandingsLeagues {
			break
		}
		if len(allowed) > 0 && !slices.Contains(allowed, league.ExternalID) {
			continue
		}
		calls++
		rows, err := standings.Standings(ctx, league.ExternalID, league.Season)
		if err != nil {
			errList = append(errList, err)
			if errors.Is(err, errs.ErrQuotaExceeded) {
				break
			}
			continue
		}
		strengths := make(map[string]float64, len(rows))
		for _, row := range rows {
			strengths[row.TeamExternalID] = domain.StrengthFromGoals(row.GoalsFor, row.GoalsAgainst)
		}
		n, err := s.repository.UpdateTeamStrengths(ctx, provider, strengths)
		if err != nil {
			errList = append(errList, err)
			continue
		}
		updated += n
	}
	return calls, updated, errors.Join(errList...)
}
