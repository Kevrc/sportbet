// Package app tiene las dependencias que main.go entrega a cada módulo.
package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sportbet/internal/shared/config"
	"sportbet/internal/shared/contracts"
	"sportbet/internal/shared/eventbus"
)

// Deps es lo que recibe el Register de cada módulo. Los contratos ya vienen
// construidos: el real si el dueño lo entregó, el fake si todavía no.
type Deps struct {
	DB        *pgxpool.Pool
	Bus       eventbus.Bus
	Config    config.Config
	Log       *slog.Logger
	Clock     func() time.Time // inyectable para el ReplayProvider acelerado y las pruebas
	Lifecycle *Lifecycle

	Auth     contracts.Authenticator
	Markets  contracts.MarketReader
	Results  contracts.ResultReader
	Wallet   contracts.WalletService
	Fixtures contracts.FixtureProvider
	Live     contracts.LiveScoreProvider
}

// Lifecycle arranca los workers de fondo (Sync Worker, barrido de settlement)
// y los detiene cuando se cancela el contexto.
type Lifecycle struct {
	log *slog.Logger
	mu  sync.Mutex
	fns []worker
}

type worker struct {
	name string
	fn   func(ctx context.Context) error
}

func NewLifecycle(log *slog.Logger) *Lifecycle { return &Lifecycle{log: log} }

// Go registra un worker. Se ejecuta cuando main llama a Run.
func (l *Lifecycle) Go(name string, fn func(ctx context.Context) error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fns = append(l.fns, worker{name, fn})
}

// Run ejecuta todos los workers y espera a que terminen al cancelar ctx.
func (l *Lifecycle) Run(ctx context.Context) {
	l.mu.Lock()
	fns := append([]worker(nil), l.fns...)
	l.mu.Unlock()
	var wg sync.WaitGroup
	for _, w := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.log.Info("worker iniciado", "worker", w.name)
			if err := w.fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
				l.log.Error("worker terminó con error", "worker", w.name, "err", err)
			}
		}()
	}
	wg.Wait()
}
