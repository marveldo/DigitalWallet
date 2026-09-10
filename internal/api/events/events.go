package events

import (
	"context"
	"log/slog"
	"sync"
)

// Handler reacts to one published event. Handlers run synchronously inside the
// caller's goroutine, so they must stay fast — the work itself belongs on a
// queue. A handler that blocks blocks the HTTP request that published.
type Handler func(ctx context.Context, payload any)

type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	Logger   *slog.Logger
}

type EventBusConfig struct {
	Logger *slog.Logger
}

func NewEventBus(cfg *EventBusConfig) *EventBus {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBus{
		handlers: make(map[string][]Handler),
		Logger:   logger.With(slog.String("layer", "events")),
	}
}

// Subscribe registers a handler for an event name. Several handlers may listen
// to the same event; they run in registration order.
func (b *EventBus) Subscribe(eventName string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
	b.Logger.Debug("event handler subscribed", slog.String("event", eventName))
}

// Publish delivers the payload to every handler subscribed to eventName.
//
// It returns nothing: publishing is a notification, not a request. A listener
// that fails must not fail the business operation that emitted the event — a
// user is still created even when the welcome email cannot be queued.
func (b *EventBus) Publish(ctx context.Context, eventName string, payload any) {
	b.mu.RLock()
	// Copy the slice so a handler that subscribes during dispatch cannot
	// mutate what we are ranging over, and so the lock is not held while
	// handlers run.
	handlers := make([]Handler, len(b.handlers[eventName]))
	copy(handlers, b.handlers[eventName])
	b.mu.RUnlock()

	if len(handlers) == 0 {
		b.Logger.DebugContext(ctx, "event published with no subscribers", slog.String("event", eventName))
		return
	}

	b.Logger.DebugContext(ctx, "event published",
		slog.String("event", eventName),
		slog.Int("handlers", len(handlers)),
	)
	for _, handler := range handlers {
		b.dispatch(ctx, eventName, handler, payload)
	}
}

// dispatch isolates one handler. A panicking listener is contained here rather
// than taking down the request that published the event.
func (b *EventBus) dispatch(ctx context.Context, eventName string, handler Handler, payload any) {
	defer func() {
		if r := recover(); r != nil {
			b.Logger.ErrorContext(ctx, "event handler panicked",
				slog.String("event", eventName),
				slog.Any("panic", r),
			)
		}
	}()
	handler(ctx, payload)
}
