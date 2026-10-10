package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/errs"
)

type APIFootball struct {
	baseURL *url.URL
	key     string
	client  *http.Client
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

func (p *APIFootball) getFixtures(ctx context.Context, query url.Values) (apiResponse, error) {
	var response apiResponse
	if err := p.get(ctx, "fixtures", query, &response); err != nil {
		return apiResponse{}, err
	}
	return response, nil
}

func (p *APIFootball) get(ctx context.Context, endpoint string, query url.Values, target *apiResponse) error {
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
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return errs.Wrap(errs.ErrProviderUnavailable, err, "PROVIDER_INVALID_RESPONSE", "API-Football devolvió JSON inválido")
	}
	if hasAPIErrors(target.Errors) {
		return errs.New(errs.ErrProviderUnavailable, "PROVIDER_API_ERROR",
			"API-Football informó un error en la consulta: "+string(target.Errors))
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
		},
		Home:    contracts.ProviderTeam{ExternalID: strconv.Itoa(item.Teams.Home.ID), Name: item.Teams.Home.Name},
		Away:    contracts.ProviderTeam{ExternalID: strconv.Itoa(item.Teams.Away.ID), Name: item.Teams.Away.Name},
		StartAt: item.Fixture.Date, Status: matchStatus(item.Fixture.Status.Short),
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
		ID     int       `json:"id"`
		Date   time.Time `json:"date"`
		Status struct {
			Short   string `json:"short"`
			Elapsed *int   `json:"elapsed"`
		} `json:"status"`
	} `json:"fixture"`
	League struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Country string `json:"country"`
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
