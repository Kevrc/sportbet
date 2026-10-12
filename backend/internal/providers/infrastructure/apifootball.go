package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type APIFootball struct {
	baseURL *url.URL
	key     string
	client  *http.Client

	// maxSeason es la última temporada que el plan permite consultar en
	// /standings (el gratuito no incluye la actual). 0 = sin límite conocido.
	maxSeason atomic.Int32
}

func NewAPIFootball(baseURL, key string, client *http.Client) (*APIFootball, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil, errs.New(errs.ErrValidation, "INVALID_PROVIDER_URL", "la URL de API-Football debe ser HTTP o HTTPS")
	}
	if parsed.Scheme == "http" && parsed.Hostname() != "localhost" && !isLoopback(parsed.Hostname()) {
		return nil, errs.New(errs.ErrValidation, "INSECURE_PROVIDER_URL", "API-Football requiere HTTPS fuera de loopback")
	}
	if strings.TrimSpace(key) == "" {
		return nil, errs.New(errs.ErrValidation, "MISSING_PROVIDER_KEY", "falta la clave de API-Football")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &APIFootball{baseURL: parsed, key: key, client: client}, nil
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (p *APIFootball) Name() string { return "apifootball" }

func (p *APIFootball) FixturesByDate(ctx context.Context, date time.Time, leagueIDs []string) ([]contracts.ProviderFixture, error) {
	query := url.Values{"date": {date.UTC().Format("2006-01-02")}}
	var response apiResponse
	if err := p.get(ctx, "fixtures", query, &response); err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(leagueIDs))
	for _, id := range leagueIDs {
		allowed[id] = struct{}{}
	}
	fixtures := make([]contracts.ProviderFixture, 0, len(response.Response))
	for _, item := range response.Response {
		if len(allowed) > 0 {
			if _, ok := allowed[strconv.Itoa(item.League.ID)]; !ok {
				continue
			}
		}
		fixture, err := convertFixture(item, false)
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
	}
	return fixtures, nil
}

func (p *APIFootball) FixtureResult(ctx context.Context, externalID string) (contracts.ProviderFixture, error) {
	if externalID == "" {
		return contracts.ProviderFixture{}, errs.New(errs.ErrValidation, "INVALID_FIXTURE_ID", "el ID externo del fixture es obligatorio")
	}
	response, err := p.getFixtures(ctx, url.Values{"id": {externalID}})
	if err != nil {
		return contracts.ProviderFixture{}, err
	}
	if len(response.Response) == 0 {
		return contracts.ProviderFixture{}, errs.New(errs.ErrNotFound, "FIXTURE_NOT_FOUND", "fixture no encontrado en API-Football")
	}
	return convertFixture(response.Response[0], true)
}

func (p *APIFootball) LiveScores(ctx context.Context) ([]contracts.LiveScore, error) {
	response, err := p.getFixtures(ctx, url.Values{"live": {"all"}})
	if err != nil {
		return nil, err
	}
	scores := make([]contracts.LiveScore, 0, len(response.Response))
	for _, item := range response.Response {
		if !validMatchStatus(item.Fixture.Status.Short) {
			return nil, errs.New(errs.ErrProviderUnavailable, "PROVIDER_INVALID_RESPONSE", "API-Football devolvió un estado desconocido")
		}
		status := matchStatus(item.Fixture.Status.Short)
		if status != contracts.MatchLive && status != contracts.MatchInterrupted {
			continue
		}
		if item.Goals.Home == nil || item.Goals.Away == nil {
			return nil, errs.New(errs.ErrProviderUnavailable, "PROVIDER_INVALID_RESPONSE", "API-Football no entregó el marcador en vivo")
		}
		minute := 0
		if item.Fixture.Status.Elapsed != nil {
			minute = *item.Fixture.Status.Elapsed
		}
		scores = append(scores, contracts.LiveScore{
			ExternalID: strconv.Itoa(item.Fixture.ID),
			Status:     status, HomeScore: *item.Goals.Home, AwayScore: *item.Goals.Away,
			Minute: minute,
		})
	}
	return scores, nil
}

// Standings devuelve la tabla de la liga y temporada. Algunas ligas (como la
// Liga 1 de Perú) tienen varias tablas (Apertura, Clausura, acumulada): de cada
// equipo se toma la fila con más partidos jugados.
func (p *APIFootball) Standings(ctx context.Context, leagueExternalID string, season int) ([]contracts.TeamStanding, error) {
	if leagueExternalID == "" || season <= 0 {
		return nil, errs.New(errs.ErrValidation, "INVALID_STANDINGS_QUERY", "la liga y la temporada son obligatorias")
	}
	if limit := int(p.maxSeason.Load()); limit > 0 && season > limit {
		season = limit
	}
	var response apiStandingsResponse
	query := url.Values{"league": {leagueExternalID}, "season": {strconv.Itoa(season)}}
	err := p.get(ctx, "standings", query, &response)
	if limit := planMaxSeason(err); limit > 0 && limit < season {
		// El plan gratuito responde "try from 2022 to 2024": se usa la última
		// temporada permitida como fuerza histórica y se recuerda el límite
		// para no gastar otra llamada en las siguientes ligas.
		p.maxSeason.Store(int32(limit))
		query.Set("season", strconv.Itoa(limit))
		response = apiStandingsResponse{}
		err = p.get(ctx, "standings", query, &response)
	}
	if err != nil {
		return nil, err
	}
	best := map[int]contracts.TeamStanding{}
	for _, item := range response.Response {
		for _, group := range item.League.Standings {
			for _, row := range group {
				if row.Team.ID <= 0 {
					continue
				}
				current, seen := best[row.Team.ID]
				if seen && current.Played >= row.All.Played {
					continue
				}
				best[row.Team.ID] = contracts.TeamStanding{
					TeamExternalID: strconv.Itoa(row.Team.ID),
					Played:         row.All.Played,
					GoalsFor:       row.All.Goals.For,
					GoalsAgainst:   row.All.Goals.Against,
				}
			}
		}
	}
	out := make([]contracts.TeamStanding, 0, len(best))
	for _, standing := range best {
		out = append(out, standing)
	}
	return out, nil
}

var planSeasonRange = regexp.MustCompile(`(?i)season.*try from \d{4} to (\d{4})`)

// planMaxSeason extrae la última temporada permitida del error del plan, o 0.
func planMaxSeason(err error) int {
	if err == nil {
		return 0
	}
	m := planSeasonRange.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	year, _ := strconv.Atoi(m[1])
	return year
}

type apiStandingsResponse struct {
	Response []struct {
		League struct {
			Standings [][]struct {
				Team struct {
					ID int `json:"id"`
				} `json:"team"`
				All struct {
					Played int `json:"played"`
					Goals  struct {
						For     int `json:"for"`
						Against int `json:"against"`
					} `json:"goals"`
				} `json:"all"`
			} `json:"standings"`
		} `json:"league"`
	} `json:"response"`
}

func (p *APIFootball) getFixtures(ctx context.Context, query url.Values) (apiResponse, error) {
	var response apiResponse
	if err := p.get(ctx, "fixtures", query, &response); err != nil {
		return apiResponse{}, err
	}
	return response, nil
}

func (p *APIFootball) get(ctx context.Context, endpoint string, query url.Values, target any) error {
	requestURL := *p.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + "/" + endpoint
	requestURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return errs.Wrap(errs.ErrProviderUnavailable, err, "PROVIDER_REQUEST_FAILED", "no se pudo preparar la consulta a API-Football")
	}
	request.Header.Set("x-apisports-key", p.key)
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return errs.Wrap(errs.ErrProviderUnavailable, err, "PROVIDER_UNAVAILABLE", "no se pudo consultar API-Football")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		return errs.New(errs.ErrQuotaExceeded, "PROVIDER_QUOTA_EXCEEDED", "API-Football rechazó la consulta por límite de cuota")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errs.New(errs.ErrProviderUnavailable, "PROVIDER_HTTP_ERROR", fmt.Sprintf("API-Football respondió HTTP %d", response.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 20<<20))
	if err != nil {
		return errs.Wrap(errs.ErrProviderUnavailable, err, "PROVIDER_UNAVAILABLE", "no se pudo leer la respuesta de API-Football")
	}
	var envelope struct {
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return errs.Wrap(errs.ErrProviderUnavailable, err, "PROVIDER_INVALID_RESPONSE", "API-Football devolvió JSON inválido")
	}
	if hasAPIErrors(envelope.Errors) {
		return errs.New(errs.ErrProviderUnavailable, "PROVIDER_API_ERROR",
			"API-Football informó un error en la consulta: "+string(envelope.Errors))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return errs.Wrap(errs.ErrProviderUnavailable, err, "PROVIDER_INVALID_RESPONSE", "API-Football devolvió JSON inválido")
	}
	return nil
}

// hasAPIErrors es falso para ausente, null, [] y {}.
func hasAPIErrors(raw json.RawMessage) bool {
	switch strings.TrimSpace(string(raw)) {
	case "", "null", "[]", "{}":
		return false
	}
	return true
}

// safeLogoURL solo acepta URLs https: el frontend las usa como src de <img>.
func safeLogoURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}

func convertFixture(item apiFixture, useFulltime bool) (contracts.ProviderFixture, error) {
	if item.Fixture.ID <= 0 || item.League.ID <= 0 ||
		item.Teams.Home.ID <= 0 || item.Teams.Away.ID <= 0 ||
		!validMatchStatus(item.Fixture.Status.Short) {
		return contracts.ProviderFixture{}, errs.New(errs.ErrProviderUnavailable, "PROVIDER_INVALID_RESPONSE", "API-Football devolvió un fixture incompleto o un estado desconocido")
	}
	fixture := contracts.ProviderFixture{
		ExternalID: strconv.Itoa(item.Fixture.ID),
		League: contracts.ProviderLeague{
			ExternalID: strconv.Itoa(item.League.ID), Name: item.League.Name, Country: item.League.Country,
			Season: item.League.Season, LogoURL: safeLogoURL(item.League.Logo), FlagURL: safeLogoURL(item.League.Flag),
		},
		Round:     item.League.Round,
		VenueName: item.Fixture.Venue.Name,
		VenueCity: item.Fixture.Venue.City,
		Home:      contracts.ProviderTeam{ExternalID: strconv.Itoa(item.Teams.Home.ID), Name: item.Teams.Home.Name, LogoURL: safeLogoURL(item.Teams.Home.Logo)},
		Away:      contracts.ProviderTeam{ExternalID: strconv.Itoa(item.Teams.Away.ID), Name: item.Teams.Away.Name, LogoURL: safeLogoURL(item.Teams.Away.Logo)},
		StartAt:   item.Fixture.Date, Status: matchStatus(item.Fixture.Status.Short),
	}
	if useFulltime {
		fixture.HomeScore, fixture.AwayScore = item.Score.Fulltime.Home, item.Score.Fulltime.Away
	} else if item.Score.Fulltime.Home != nil && item.Score.Fulltime.Away != nil {
		fixture.HomeScore, fixture.AwayScore = item.Score.Fulltime.Home, item.Score.Fulltime.Away
	}
	if fixture.Status == contracts.MatchFinished && (fixture.HomeScore == nil || fixture.AwayScore == nil) {
		return contracts.ProviderFixture{}, errs.New(errs.ErrProviderUnavailable, "PROVIDER_RESULT_MISSING", "API-Football no entregó el marcador de tiempo reglamentario")
	}
	return fixture, nil
}

type apiResponse struct {
	// API-Football manda "errors": [] cuando todo salió bien y un objeto
	// {"campo": "mensaje"} cuando hay errores; RawMessage acepta ambos.
	Errors   json.RawMessage `json:"errors"`
	Response []apiFixture    `json:"response"`
}

type apiFixture struct {
	Fixture struct {
		ID    int       `json:"id"`
		Date  time.Time `json:"date"`
		Venue struct {
			Name string `json:"name"`
			City string `json:"city"`
		} `json:"venue"`
		Status struct {
			Short   string `json:"short"`
			Elapsed *int   `json:"elapsed"`
		} `json:"status"`
	} `json:"fixture"`
	League struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Country string `json:"country"`
		Season  int    `json:"season"`
		Logo    string `json:"logo"`
		Flag    string `json:"flag"`
		Round   string `json:"round"`
	} `json:"league"`
	Teams struct {
		Home apiTeam `json:"home"`
		Away apiTeam `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home *int `json:"home"`
		Away *int `json:"away"`
	} `json:"goals"`
	Score struct {
		Fulltime struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"fulltime"`
	} `json:"score"`
}

type apiTeam struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Logo   string `json:"logo"`
	Winner *bool  `json:"winner"`
}

var (
	_ contracts.FixtureProvider   = (*APIFootball)(nil)
	_ contracts.LiveScoreProvider = (*APIFootball)(nil)
)

var _ contracts.StandingsProvider = (*APIFootball)(nil)
