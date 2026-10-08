// Package eventbus es el bus de eventos en memoria entre módulos.
// Se llama eventbus para no chocar con el módulo events.
//
// Reglas: publicar solo después del COMMIT; los consumidores deben ser
// idempotentes, porque el bus puede perder eventos (búfer lleno o reinicio)
// y settlement tiene un barrido de respaldo.
package eventbus

import "context"

type Event interface{ Topic() Topic }

type Handler func(ctx context.Context, e Event)

type Bus interface {
	// Publish no bloquea.
	Publish(ctx context.Context, e Event)
	// Subscribe registra un handler con un nombre para los logs y devuelve
	// la función para cancelar la suscripción.
	Subscribe(t Topic, name string, h Handler) (unsubscribe func())
}

// On es un Subscribe tipado:
//
//	eventbus.On(bus, "settlement", func(ctx context.Context, e eventbus.MatchFinished) { ... })
func On[T Event](b Bus, name string, h func(context.Context, T)) func() {
	var zero T
	return b.Subscribe(zero.Topic(), name, func(ctx context.Context, e Event) {
		if v, ok := e.(T); ok {
			h(ctx, v)
		}
	})
}
