package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Handler func(ctx context.Context, payload any)

type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	Logger   *slog.Logger
	Tracer   trace.Tracer
}

type EventBusConfig struct {
	Logger *slog.Logger
	Tracer trace.Tracer
}

func (b *EventBus) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if b == nil || b.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return b.Tracer.Start(ctx, name)
}

func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func NewEventBus(cfg *EventBusConfig) *EventBus {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBus{
		handlers: make(map[string][]Handler),
		Logger:   logger.With(slog.String("layer", "events")),
		Tracer:   cfg.Tracer,
	}
}

func (b *EventBus) Subscribe(eventName string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], handler)
	b.Logger.Debug("event handler subscribed", slog.String("event", eventName))
}

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

func (b *EventBus) dispatch(ctx context.Context, eventName string, handler Handler, payload any) {
	ctx, span := b.StartSpan(ctx, "event."+eventName)
	defer span.End()
	defer func() {
		if r := recover(); r != nil {
			b.Logger.ErrorContext(ctx, "event handler panicked",
				slog.String("event", eventName),
				slog.Any("panic", r),
			)
			RecordError(span, fmt.Errorf("event handler panicked: %v", r))
		}
	}()
	handler(ctx, payload)
}
