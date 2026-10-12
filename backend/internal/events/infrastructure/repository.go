package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/events/application"
	"sportbet/internal/events/domain"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListLeagues(ctx context.Context) ([]application.League, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, name, country, featured, logo_url, flag_url
		FROM leagues ORDER BY featured DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("events: listar ligas: %w", err)
	}
	defer rows.Close()
	leagues := make([]application.League, 0)
	for rows.Next() {
		var league application.League
		if err := rows.Scan(&league.ID, &league.Name, &league.Country, &league.Featured, &league.LogoURL, &league.FlagURL); err != nil {
			return nil, fmt.Errorf("events: leer liga: %w", err)
		}
		leagues = append(leagues, league)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events: recorrer ligas: %w", err)
	}
	return leagues, nil
}

func (r *Repository) ListEvents(ctx context.Context, filter application.Filter) ([]application.Event, error) {
	var dateStart, dateEnd *time.Time
	if filter.Date != nil {
		start := time.Date(filter.Date.Year(), filter.Date.Month(), filter.Date.Day(), 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 0, 1)
		dateStart, dateEnd = &start, &end
	}
	var status string
	if filter.Status != "" {
		status = string(filter.Status)
	}
	rows, err := r.pool.Query(ctx, eventSelect+`
		WHERE ($1::timestamptz IS NULL OR e.start_at >= $1)
		  AND ($2::timestamptz IS NULL OR e.start_at < $2)
		  AND ($3::uuid IS NULL OR e.league_id = $3::uuid)
		  AND ($4::text = '' OR e.status = $4)
		  AND ($5::boolean IS NULL OR e.featured = $5)
		  AND (NOT $6::boolean OR e.status IN ('LIVE', 'INTERRUPTED'))
		ORDER BY e.start_at, e.id`,
		dateStart, dateEnd, emptyAsNil(filter.LeagueID), status, filter.Featured, filter.LiveOnly)
	if err != nil {
		return nil, fmt.Errorf("events: listar eventos: %w", err)
	}
	defer rows.Close()
	events := make([]application.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events: recorrer eventos: %w", err)
	}
	return events, nil
}

func (r *Repository) GetEvent(ctx context.Context, eventID string) (application.Event, error) {
	event, err := scanEvent(r.pool.QueryRow(ctx, eventSelect+`WHERE e.id = $1::uuid`, eventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Event{}, errs.New(errs.ErrNotFound, "EVENT_NOT_FOUND", "evento no encontrado")
	}
	if err != nil {
		return application.Event{}, fmt.Errorf("events: consultar evento: %w", err)
	}
	event.Markets, err = listEventMarkets(ctx, r.pool, eventID)
	return event, err
}

func (r *Repository) SetFeatured(ctx context.Context, eventID string, featured bool) (application.Event, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE events SET featured = $2 WHERE id = $1::uuid`, eventID, featured)
	if err != nil {
		return application.Event{}, fmt.Errorf("events: actualizar evento destacado: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return application.Event{}, errs.New(errs.ErrNotFound, "EVENT_NOT_FOUND", "evento no encontrado")
	}
	return r.GetEvent(ctx, eventID)
}

func listEventMarkets(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, eventID string) ([]application.Market, error) {
	rows, err := q.Query(ctx, `
		SELECT m.id::text, m.event_id::text, m.code, m.status,
		       s.id::text, s.code, s.label, s.odd_x100, s.result
		FROM markets m
		LEFT JOIN selections s ON s.market_id = m.id
		WHERE m.event_id = $1::uuid
		ORDER BY m.code, array_position(ARRAY['HOME','DRAW','AWAY','OVER','UNDER','YES','NO'], s.code)`, eventID)
	if err != nil {
		return nil, fmt.Errorf("events: consultar mercados del evento: %w", err)
	}
	defer rows.Close()
	markets := make([]application.Market, 0)
	positions := make(map[string]int)
	for rows.Next() {
		var marketID, eventKey, code, status string
		var selectionID, selectionCode, label, result *string
		var oddX100 *int32
		if err := rows.Scan(&marketID, &eventKey, &code, &status, &selectionID, &selectionCode, &label, &oddX100, &result); err != nil {
			return nil, fmt.Errorf("events: leer mercado del evento: %w", err)
		}
		position, ok := positions[marketID]
		if !ok {
			position = len(markets)
			positions[marketID] = position
			markets = append(markets, application.Market{
				ID: marketID, EventID: eventKey, Code: code, Status: contracts.MarketStatus(status),
				Selections: make([]application.Selection, 0),
			})
		}
		if selectionID != nil {
			markets[position].Selections = append(markets[position].Selections, application.Selection{
				ID: *selectionID, Code: value(selectionCode), Label: value(label),
				OddX100: int64(value(oddX100)), Result: contracts.SelectionOutcome(value(result)),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events: recorrer mercados del evento: %w", err)
	}
	return markets, nil
}

func (r *Repository) ImportFixtures(ctx context.Context, provider string, now time.Time, fixtures []contracts.ProviderFixture, refresh bool) ([]application.Change, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("events: iniciar importación de fixtures: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	changes := make([]application.Change, 0)
	for _, fixture := range fixtures {
		leagueID, err := upsertLeague(ctx, tx, provider, fixture.League)
		if err != nil {
			return nil, err
		}
		homeID, err := upsertTeam(ctx, tx, provider, fixture.Home)
		if err != nil {
			return nil, err
		}
		awayID, err := upsertTeam(ctx, tx, provider, fixture.Away)
		if err != nil {
			return nil, err
		}
		change, err := r.upsertFixture(ctx, tx, provider, now, fixture, leagueID, homeID, awayID, refresh)
		if err != nil {
			return nil, err
		}
		if change != nil {
			changes = append(changes, *change)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("events: confirmar importación de fixtures: %w", err)
	}
	return changes, nil
}

func (r *Repository) ListResultCandidates(ctx context.Context, provider string, cutoff time.Time) ([]application.ResultCandidate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT x.external_id, e.start_at
		FROM external_refs x
		JOIN events e ON e.id = x.entity_id
		WHERE x.provider = $1 AND x.entity_type = 'event'
		  AND e.status IN ('SCHEDULED', 'LIVE', 'INTERRUPTED')
		  AND e.start_at <= $2
		  AND e.start_at >= $2 - INTERVAL '36 hours'
		  AND (e.live_updated_at IS NULL OR e.live_updated_at <= $2)
		ORDER BY e.start_at
		LIMIT 100`, provider, cutoff)
	if err != nil {
		return nil, fmt.Errorf("events: buscar resultados pendientes: %w", err)
	}
	defer rows.Close()
	ids := make([]application.ResultCandidate, 0)
	for rows.Next() {
		var c application.ResultCandidate
		if err := rows.Scan(&c.ExternalID, &c.StartAt); err != nil {
			return nil, fmt.Errorf("events: leer referencia pendiente: %w", err)
		}
		ids = append(ids, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("events: recorrer resultados pendientes: %w", err)
	}
	return ids, nil
}

func (r *Repository) ApplyLiveScores(ctx context.Context, provider string, now time.Time, scores []contracts.LiveScore) ([]application.Change, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("events: iniciar actualización de marcadores: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	changes := make([]application.Change, 0, len(scores))
	for _, score := range scores {
		var eventID string
		err := tx.QueryRow(ctx, `
			SELECT e.id::text
			FROM external_refs x JOIN events e ON e.id = x.entity_id
			WHERE x.provider = $1 AND x.entity_type = 'event' AND x.external_id = $2`, provider, score.ExternalID).
			Scan(&eventID)
		if errors.Is(err, pgx.ErrNoRows) {
			// fixtures?live=all trae todos los partidos en juego del mundo; los
			// de ligas que no importamos se ignoran sin descartar el resto.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("events: localizar evento del marcador: %w", err)
		}
		var previous contracts.MatchStatus
		var liveUpdatedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT status, live_updated_at FROM events WHERE id = $1::uuid FOR UPDATE`, eventID).
			Scan(&previous, &liveUpdatedAt); err != nil {
			return nil, fmt.Errorf("events: leer estado antes del marcador: %w", err)
		}
		observedAt := score.ObservedAt
		if observedAt.IsZero() {
			observedAt = now
		}
		if liveUpdatedAt != nil && observedAt.Before(*liveUpdatedAt) {
			continue
		}
		if !domain.CanTransition(previous, score.Status, false) {
			continue
		}
		_, err = tx.Exec(ctx, `
			UPDATE events SET status = $2, home_score = $3, away_score = $4,
			                   minute = $5, live_updated_at = $6,
			                   kicked_off_at = COALESCE(kicked_off_at, $6)
			WHERE id = $1::uuid`, eventID, score.Status, score.HomeScore, score.AwayScore, score.Minute, observedAt)
		if err != nil {
			return nil, fmt.Errorf("events: guardar marcador en vivo: %w", err)
		}
		event, err := scanEvent(tx.QueryRow(ctx, eventSelect+`WHERE e.id = $1::uuid`, eventID))
		if err != nil {
			return nil, fmt.Errorf("events: leer evento actualizado: %w", err)
		}
		changes = append(changes, application.Change{
			Event: event, PreviousStatus: previous,
			Started:   previous != contracts.MatchLive && previous != contracts.MatchInterrupted,
			UpdatedAt: observedAt,
		})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("events: confirmar marcadores en vivo: %w", err)
	}
	return changes, nil
}

func (r *Repository) upsertFixture(
	ctx context.Context,
	tx pgx.Tx,
	provider string,
	now time.Time,
	fixture contracts.ProviderFixture,
	leagueID, homeID, awayID string,
	refresh bool,
) (*application.Change, error) {
	if err := lockExternal(ctx, tx, provider, "event", fixture.ExternalID); err != nil {
		return nil, err
	}
	var eventID string
	err := tx.QueryRow(ctx, `
		SELECT entity_id::text FROM external_refs
		WHERE provider = $1 AND entity_type = 'event' AND external_id = $2`, provider, fixture.ExternalID).Scan(&eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		var homeScore, awayScore *int
		homeScore, awayScore = fixture.HomeScore, fixture.AwayScore
		var startedAt, finishedAt, liveUpdatedAt *time.Time
		if fixture.Status == contracts.MatchLive || fixture.Status == contracts.MatchInterrupted {
			startedAt = &fixture.StartAt
			liveUpdatedAt = &now
		}
		if fixture.Status.IsTerminal() {
			finishedAt = &now
		}
		if fixture.Status == contracts.MatchFinished {
			startedAt = &fixture.StartAt
		}
		if refresh {
			liveUpdatedAt = &now
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO events (league_id, home_team_id, away_team_id, start_at, status,
			                    home_score, away_score, kicked_off_at, live_updated_at, finished_at,
			                    round, venue_name, venue_city)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10,
			        NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''))
			RETURNING id::text`, leagueID, homeID, awayID, fixture.StartAt, fixture.Status,
			homeScore, awayScore, startedAt, liveUpdatedAt, finishedAt,
			fixture.Round, fixture.VenueName, fixture.VenueCity).Scan(&eventID)
		if err != nil {
			return nil, fmt.Errorf("events: insertar evento: %w", err)
		}
		if err := insertExternalRef(ctx, tx, provider, "event", eventID, fixture.ExternalID); err != nil {
			return nil, err
		}
		event, err := scanEvent(tx.QueryRow(ctx, eventSelect+`WHERE e.id = $1::uuid`, eventID))
		if err != nil {
			return nil, fmt.Errorf("events: leer evento importado: %w", err)
		}
		return &application.Change{
			Event: event, PreviousStatus: "", Started: startedAt != nil,
			Finished: finishedAt != nil, UpdatedAt: now,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("events: localizar fixture importado: %w", err)
	}
	var previous contracts.MatchStatus
	var previousHomeScore, previousAwayScore *int32
	if err := tx.QueryRow(ctx, `
		SELECT status, home_score, away_score FROM events WHERE id = $1::uuid FOR UPDATE`,
		eventID).Scan(&previous, &previousHomeScore, &previousAwayScore); err != nil {
		return nil, fmt.Errorf("events: leer estado del fixture: %w", err)
	}
	if !domain.CanTransition(previous, fixture.Status, false) {
		return nil, nil
	}
	started := previous != contracts.MatchLive && previous != contracts.MatchInterrupted &&
		(fixture.Status == contracts.MatchLive || fixture.Status == contracts.MatchInterrupted)
	finished := !previous.IsTerminal() && fixture.Status.IsTerminal()
	scoreChanged := scoreDiffers(previousHomeScore, fixture.HomeScore) || scoreDiffers(previousAwayScore, fixture.AwayScore)
	_, err = tx.Exec(ctx, `
		UPDATE events SET league_id = $2::uuid, home_team_id = $3::uuid, away_team_id = $4::uuid,
		                  start_at = $5, status = $6,
		                  home_score = COALESCE($7, home_score), away_score = COALESCE($8, away_score),
		                  kicked_off_at = CASE
		                      WHEN $9::boolean THEN COALESCE(kicked_off_at, $14)
		                      WHEN $11::boolean AND $13::boolean THEN COALESCE(kicked_off_at, $14)
		                      ELSE kicked_off_at END,
		                  finished_at = CASE WHEN $11::boolean THEN COALESCE(finished_at, $10) ELSE finished_at END,
		                  live_updated_at = CASE WHEN $12::boolean THEN $10
		                      WHEN $9::boolean THEN COALESCE(live_updated_at, $10)
		                      ELSE live_updated_at END,
		                  round = COALESCE(NULLIF($15, ''), round),
		                  venue_name = COALESCE(NULLIF($16, ''), venue_name),
		                  venue_city = COALESCE(NULLIF($17, ''), venue_city)
		WHERE id = $1::uuid`, eventID, leagueID, homeID, awayID, fixture.StartAt, fixture.Status,
		fixture.HomeScore, fixture.AwayScore, started, now, fixture.Status.IsTerminal(), refresh,
		fixture.Status == contracts.MatchFinished, fixture.StartAt,
		fixture.Round, fixture.VenueName, fixture.VenueCity)
	if err != nil {
		return nil, fmt.Errorf("events: actualizar fixture: %w", err)
	}
	event, err := scanEvent(tx.QueryRow(ctx, eventSelect+`WHERE e.id = $1::uuid`, eventID))
	if err != nil {
		return nil, fmt.Errorf("events: leer fixture actualizado: %w", err)
	}
	if previous == fixture.Status && !started && !finished && !scoreChanged {
		return nil, nil
	}
	return &application.Change{
		Event: event, PreviousStatus: previous,
		Started: started, Finished: finished, UpdatedAt: now,
	}, nil
}

func scoreDiffers(previous *int32, current *int) bool {
	if current == nil {
		return false
	}
	return previous == nil || int(*previous) != *current
}

func upsertLeague(ctx context.Context, tx pgx.Tx, provider string, league contracts.ProviderLeague) (string, error) {
	if err := lockExternal(ctx, tx, provider, "league", league.ExternalID); err != nil {
		return "", err
	}
	var id string
	err := tx.QueryRow(ctx, `
		SELECT entity_id::text FROM external_refs
		WHERE provider = $1 AND entity_type = 'league' AND external_id = $2`, provider, league.ExternalID).Scan(&id)
	if err == nil {
		_, err = tx.Exec(ctx, `
			UPDATE leagues SET name = $2, country = $3,
			       season = COALESCE(NULLIF($4, 0), season),
			       logo_url = COALESCE(NULLIF($5, ''), logo_url),
			       flag_url = COALESCE(NULLIF($6, ''), flag_url)
			WHERE id = $1::uuid`, id, league.Name, league.Country, league.Season, league.LogoURL, league.FlagURL)
		if err != nil {
			return "", fmt.Errorf("events: actualizar liga: %w", err)
		}
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("events: localizar liga: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO leagues (name, country, season, logo_url, flag_url)
		VALUES ($1, $2, NULLIF($3, 0), NULLIF($4, ''), NULLIF($5, ''))
		RETURNING id::text`, league.Name, league.Country, league.Season, league.LogoURL, league.FlagURL).Scan(&id); err != nil {
		return "", fmt.Errorf("events: insertar liga: %w", err)
	}
	return id, insertExternalRef(ctx, tx, provider, "league", id, league.ExternalID)
}

func upsertTeam(ctx context.Context, tx pgx.Tx, provider string, team contracts.ProviderTeam) (string, error) {
	if err := lockExternal(ctx, tx, provider, "team", team.ExternalID); err != nil {
		return "", err
	}
	var id string
	err := tx.QueryRow(ctx, `
		SELECT entity_id::text FROM external_refs
		WHERE provider = $1 AND entity_type = 'team' AND external_id = $2`, provider, team.ExternalID).Scan(&id)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE teams SET name = $2, logo_url = COALESCE(NULLIF($3, ''), logo_url) WHERE id = $1::uuid`,
			id, team.Name, team.LogoURL)
		if err != nil {
			return "", fmt.Errorf("events: actualizar equipo: %w", err)
		}
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("events: localizar equipo: %w", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO teams (name, logo_url) VALUES ($1, NULLIF($2, '')) RETURNING id::text`,
		team.Name, team.LogoURL).Scan(&id); err != nil {
		return "", fmt.Errorf("events: insertar equipo: %w", err)
	}
	return id, insertExternalRef(ctx, tx, provider, "team", id, team.ExternalID)
}

func lockExternal(ctx context.Context, tx pgx.Tx, provider, entityType, externalID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		provider+":"+entityType+":"+externalID)
	if err != nil {
		return fmt.Errorf("events: bloquear referencia externa: %w", err)
	}
	return nil
}

func insertExternalRef(ctx context.Context, tx pgx.Tx, provider, entityType, entityID, externalID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO external_refs (provider, entity_type, entity_id, external_id)
		VALUES ($1, $2, $3::uuid, $4)`, provider, entityType, entityID, externalID)
	if err != nil {
		return fmt.Errorf("events: guardar referencia externa: %w", err)
	}
	return nil
}

const eventSelect = `
	SELECT e.id::text, l.id::text, l.name, l.country, l.featured, l.logo_url, l.flag_url,
	       home.id::text, home.name, home.logo_url, away.id::text, away.name, away.logo_url,
	       e.start_at, e.status, e.home_score, e.away_score, e.minute,
	       e.featured, e.live_updated_at, e.round, e.venue_name, e.venue_city
	FROM events e
	JOIN leagues l ON l.id = e.league_id
	JOIN teams home ON home.id = e.home_team_id
	JOIN teams away ON away.id = e.away_team_id
	`

type scanner interface {
	Scan(...any) error
}

func scanEvent(row scanner) (application.Event, error) {
	var event application.Event
	var status string
	var homeScore, awayScore, minute *int32
	err := row.Scan(
		&event.ID, &event.League.ID, &event.League.Name, &event.League.Country, &event.League.Featured,
		&event.League.LogoURL, &event.League.FlagURL,
		&event.Home.ID, &event.Home.Name, &event.Home.LogoURL, &event.Away.ID, &event.Away.Name, &event.Away.LogoURL,
		&event.StartAt, &status, &homeScore, &awayScore, &minute, &event.Featured, &event.LiveUpdatedAt,
		&event.Round, &event.VenueName, &event.VenueCity,
	)
	if err != nil {
		return application.Event{}, err
	}
	event.Status = contracts.MatchStatus(status)
	event.HomeScore, event.AwayScore, event.Minute = int32PointerToInt(homeScore), int32PointerToInt(awayScore), int32PointerToInt(minute)
	return event, nil
}

func int32PointerToInt(value *int32) *int {
	if value == nil {
		return nil
	}
	result := int(*value)
	return &result
}

func value[T any](item *T) T {
	if item == nil {
		var zero T
		return zero
	}
	return *item
}

func emptyAsNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var _ application.Repository = (*Repository)(nil)

func (r *Repository) ListStandingsLeagues(ctx context.Context, provider string) ([]application.StandingsLeague, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT x.external_id, l.season
		FROM leagues l
		JOIN external_refs x ON x.entity_type = 'league' AND x.entity_id = l.id AND x.provider = $1
		WHERE l.season IS NOT NULL
		ORDER BY l.name`, provider)
	if err != nil {
		return nil, fmt.Errorf("events: listar ligas con temporada: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (application.StandingsLeague, error) {
		var l application.StandingsLeague
		err := row.Scan(&l.ExternalID, &l.Season)
		return l, err
	})
}

// UpdateTeamStrengths actualiza teams.strength por ID externo del equipo.
// Los equipos que no importamos se ignoran.
func (r *Repository) UpdateTeamStrengths(ctx context.Context, provider string, strengths map[string]float64) (int, error) {
	if len(strengths) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(strengths))
	values := make([]float64, 0, len(strengths))
	for id, value := range strengths {
		ids = append(ids, id)
		values = append(values, value)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE teams t SET strength = v.strength
		FROM unnest($2::text[], $3::numeric[]) AS v(external_id, strength)
		JOIN external_refs x ON x.external_id = v.external_id
		WHERE x.provider = $1 AND x.entity_type = 'team' AND x.entity_id = t.id
		  AND t.strength IS DISTINCT FROM v.strength`, provider, ids, values)
	if err != nil {
		return 0, fmt.Errorf("events: actualizar fuerza de equipos: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// HasFeaturedInPlay busca un evento destacado de este proveedor en juego, o
// programado entre 3 horas atrás (por si se perdió el inicio) y 5 minutos
// adelante.
func (r *Repository) HasFeaturedInPlay(ctx context.Context, provider string, now time.Time) (bool, error) {
	var found bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM events e
			JOIN external_refs x ON x.entity_type = 'event' AND x.entity_id = e.id AND x.provider = $1
			WHERE e.featured AND (
				e.status IN ('LIVE', 'INTERRUPTED') OR
				(e.status = 'SCHEDULED' AND e.start_at BETWEEN $2::timestamptz - interval '3 hours'
				                                         AND $2::timestamptz + interval '5 minutes')
			)
		)`, provider, now).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("events: buscar destacados en juego: %w", err)
	}
	return found, nil
}
