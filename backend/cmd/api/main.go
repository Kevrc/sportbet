// Command api arranca el backend de SportBet. CONGELADO después del commit
// base: los módulos se conectan solo a través de su Register y de los
// constructores de contratos, cuyas firmas no cambian.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"sportbet/internal/admin"
	"sportbet/internal/auth"
	"sportbet/internal/betting"
	"sportbet/internal/events"
	"sportbet/internal/markets"
	"sportbet/internal/providers"
	"sportbet/internal/settlement"
	"sportbet/internal/shared/app"
	"sportbet/internal/shared/config"
	"sportbet/internal/shared/db"
	"sportbet/internal/shared/eventbus"
	"sportbet/internal/users"
	"sportbet/internal/wallet"
	"sportbet/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api terminó con error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		return err
	}

	bus := eventbus.NewInMemory(log, 256)
	defer bus.Close()
	lifecycle := app.NewLifecycle(log)

	fixtures, live := providers.New(cfg, pool, log)
	deps := app.Deps{
		DB:        pool,
		Bus:       bus,
		Config:    cfg,
		Log:       log,
		Clock:     time.Now,
		Lifecycle: lifecycle,
		Auth:      auth.NewAuthenticator(cfg),
		Markets:   markets.NewMarketReader(pool),
		Results:   markets.NewResultReader(pool),
		Wallet:    wallet.NewService(pool),
		Fixtures:  fixtures,
		Live:      live,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Idempotency-Key"},
		AllowCredentials: true,
	}))
	// Sin middleware.Timeout ni Compress globales: rompen el SSE.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Route("/api/v1", func(api chi.Router) {
		auth.Register(api, deps)
		users.Register(api, deps)
		admin.Register(api, deps)
		events.Register(api, deps)
		markets.Register(api, deps)
		providers.Register(api, deps)
		betting.Register(api, deps)
		wallet.Register(api, deps)
		settlement.Register(api, deps)
	})

	// WriteTimeout en 0 porque el SSE mantiene la conexión abierta; el
	// handler del stream pone sus propios deadlines.
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r, ReadHeaderTimeout: 10 * time.Second}

	workersDone := make(chan struct{})
	go func() { lifecycle.Run(ctx); close(workersDone) }()

	errc := make(chan error, 1)
	go func() {
		log.Info("api escuchando", "addr", cfg.HTTPAddr, "provider", cfg.Provider)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	<-workersDone
	return err
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	_ = l.UnmarshalText([]byte(level))
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
