package eventbus

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestInMemoryDeliversTypedEvents(t *testing.T) {
	bus := NewInMemory(slog.New(slog.NewTextHandler(io.Discard, nil)), 8)
	var wg sync.WaitGroup
	wg.Add(1)
	var got MatchFinished
	On(bus, "test", func(_ context.Context, e MatchFinished) {
		got = e
		wg.Done()
	})
	bus.Publish(context.Background(), MatchStarted{EventID: "ignorado"})
	bus.Publish(context.Background(), MatchFinished{EventID: "e1", HomeScore: 2})
	waitOrFail(t, &wg)
	if got.EventID != "e1" || got.HomeScore != 2 {
		t.Fatalf("evento inesperado: %+v", got)
	}
	bus.Close()
}

func TestInMemoryRecoversPanics(t *testing.T) {
	bus := NewInMemory(slog.New(slog.NewTextHandler(io.Discard, nil)), 8)
	var wg sync.WaitGroup
	wg.Add(2)
	bus.Subscribe(TopicBetPlaced, "panics", func(context.Context, Event) {
		defer wg.Done()
		panic("boom")
	})
	bus.Publish(context.Background(), BetPlaced{BetID: "1"})
	bus.Publish(context.Background(), BetPlaced{BetID: "2"})
	waitOrFail(t, &wg)
	bus.Close()
}

func TestUnsubscribe(t *testing.T) {
	bus := NewInMemory(slog.New(slog.NewTextHandler(io.Discard, nil)), 8)
	unsub := bus.Subscribe(TopicBetPlaced, "x", func(context.Context, Event) { t.Error("no debería recibir") })
	unsub()
	unsub()
	bus.Publish(context.Background(), BetPlaced{})
	bus.Close()
}

func waitOrFail(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout esperando eventos")
	}
}
