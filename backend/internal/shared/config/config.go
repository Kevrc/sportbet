// Package config carga la configuración desde variables de entorno.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                string
	HTTPAddr           string
	LogLevel           string
	DatabaseURL        string
	CORSAllowedOrigins []string

	JWTSecret           string
	JWTIssuer           string
	JWTAccessTTL        time.Duration
	JWTRefreshTTL       time.Duration
	RefreshCookieSecure bool
	AdminSeedEmail      string
	AdminSeedPassword   string

	WalletInitialBalanceCents int64

	Provider            string // replay | apifootball
	APIFootballBaseURL  string
	APIFootballKey      string
	ProviderDailyBudget int
	ProviderReserve     int
	FeaturedLeagueIDs   []string
	SyncFixturesEvery   time.Duration
	SyncLiveEvery       time.Duration
	LiveStaleAfter      time.Duration
	ReplayFile          string
	ReplaySpeed         int

	SettlementSweepEvery time.Duration
	OddsMarginPercent    int
}

func (c Config) IsDev() bool { return c.Env == "development" }

// Load lee el entorno con valores por defecto pensados para desarrollo local.
func Load() (Config, error) {
	c := Config{
		Env:                       get("APP_ENV", "development"),
		HTTPAddr:                  get("HTTP_ADDR", ":8080"),
		LogLevel:                  get("LOG_LEVEL", "info"),
		DatabaseURL:               get("DATABASE_URL", "postgres://sportbet:sportbet@localhost:5432/sportbet?sslmode=disable"),
		CORSAllowedOrigins:        list("CORS_ALLOWED_ORIGINS", "http://localhost:4200"),
		JWTSecret:                 get("JWT_SECRET", "dev-secret-cambia-esto-minimo-32-caracteres"),
		JWTIssuer:                 get("JWT_ISSUER", "sportbet"),
		JWTAccessTTL:              dur("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:             dur("JWT_REFRESH_TTL", 168*time.Hour),
		RefreshCookieSecure:       get("REFRESH_COOKIE_SECURE", "false") == "true",
		AdminSeedEmail:            get("ADMIN_SEED_EMAIL", "admin@sportbet.local"),
		AdminSeedPassword:         get("ADMIN_SEED_PASSWORD", ""),
		WalletInitialBalanceCents: int64(num("WALLET_INITIAL_BALANCE_CENTS", 100000)),
		Provider:                  get("PROVIDER", "replay"),
		APIFootballBaseURL:        get("APIFOOTBALL_BASE_URL", "https://v3.football.api-sports.io"),
		APIFootballKey:            get("APIFOOTBALL_KEY", ""),
		ProviderDailyBudget:       num("PROVIDER_DAILY_BUDGET", 100),
		ProviderReserve:           num("PROVIDER_RESERVE", 13),
		FeaturedLeagueIDs:         list("FEATURED_LEAGUE_IDS", ""),
		SyncFixturesEvery:         dur("SYNC_FIXTURES_INTERVAL", 12*time.Hour),
		SyncLiveEvery:             dur("SYNC_LIVE_INTERVAL", 3*time.Minute),
		LiveStaleAfter:            dur("LIVE_STALE_AFTER", 5*time.Minute),
		ReplayFile:                get("REPLAY_FILE", "./testdata/replay/sample_match.json"),
		ReplaySpeed:               num("REPLAY_SPEED", 60),
		SettlementSweepEvery:      dur("SETTLEMENT_SWEEP_INTERVAL", 2*time.Minute),
		OddsMarginPercent:         num("ODDS_MARGIN_PERCENT", 6),
	}
	if !c.IsDev() && len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("config: JWT_SECRET debe tener al menos 32 caracteres")
	}
	if c.Provider != "replay" && c.Provider != "apifootball" {
		return c, fmt.Errorf("config: PROVIDER debe ser replay o apifootball")
	}
	return c, nil
}

func MustLoad() Config {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}

func get(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func num(k string, def int) int {
	if v, err := strconv.Atoi(get(k, "")); err == nil {
		return v
	}
	return def
}

func dur(k string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(get(k, "")); err == nil {
		return v
	}
	return def
}

func list(k, def string) []string {
	var out []string
	for _, s := range strings.Split(get(k, def), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
