package events

import (
	"context"
	"fmt"
	"log/slog"

	"github/marveldo/eda-monolith/internal/queues"
)

func RegisterPaymentListeners(bus *EventBus, worker queues.Worker[queues.PaymentPayload]) {
	bus.Subscribe(EventPaymentWebhookReceived, func(ctx context.Context, payload any) {
		ctx, span := bus.StartSpan(ctx, "listener.payment.verify")
		defer span.End()

		event, ok := payload.(PaymentWebhookPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventPaymentWebhookReceived))
			bus.Logger.ErrorContext(ctx, "unexpected payload type for event",
				slog.String("event", EventPaymentWebhookReceived),
			)
			return
		}

		log := bus.Logger.With(
			slog.String("event", EventPaymentWebhookReceived),
			slog.String("reference", event.Reference),
			slog.String("provider", event.Provider),
		)

		task, err := worker.GenerateNewTask(queues.TaskTypePaymentWebhook, queues.PaymentPayload{
			Reference:   event.Reference,
			Provider:    event.Provider,
			Event:       event.Event,
			Status:      event.Status,
			AmountMinor: event.AmountMinor,
			Currency:    event.Currency,
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build payment webhook task", slog.Any("error", err))
			RecordError(span, err)
			return
		}

		if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			log.ErrorContext(ctx, "payment webhook was not queued", slog.Any("error", err))
			RecordError(span, err)
		}
	})

	bus.Subscribe(EventPaymentInitialized, func(ctx context.Context, payload any) {
		ctx, span := bus.StartSpan(ctx, "listener.payment.initialized")
		defer span.End()

		event, ok := payload.(PaymentInitializedPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventPaymentInitialized))
			bus.Logger.ErrorContext(ctx, "unexpected payload type for event",
				slog.String("event", EventPaymentInitialized),
			)
			return
		}

		bus.Logger.InfoContext(ctx, "deposit checkout started",
			slog.String("event", EventPaymentInitialized),
			slog.String("user_id", event.UserID),
			slog.String("reference", event.Reference),
			slog.Int64("amount_minor", event.AmountMinor),
			slog.String("currency", event.Currency),
		)

		// Start polling the provider now, so the deposit still resolves if
		// its webhook never arrives.
		task, err := worker.GenerateNewTask(queues.TaskTypePaymentPoll, queues.PaymentPayload{
			Reference: event.Reference,
			Provider:  event.Provider,
		})
		if err != nil {
			bus.Logger.ErrorContext(ctx, "could not build payment poll task", slog.Any("error", err))
			RecordError(span, err)
			return
		}
		if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			bus.Logger.ErrorContext(ctx, "payment poll was not queued", slog.Any("error", err))
			RecordError(span, err)
		}
	})
}

func RegisterPaymentActivityListeners(bus *EventBus, worker queues.Worker[queues.ActivityPayload]) {
	bus.Subscribe(EventPaymentInitialized, func(ctx context.Context, payload any) {
		ctx, span := bus.StartSpan(ctx, "listener.payment.activity")
		defer span.End()

		event, ok := payload.(PaymentInitializedPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventPaymentInitialized))
			return
		}

		log := bus.Logger.With(
			slog.String("event", EventPaymentInitialized),
			slog.String("user_id", event.UserID),
		)

		task, err := worker.GenerateNewTask(queues.UpdateUserActivity, queues.ActivityPayload{
			UserID: event.UserID,
			Action: "Started a deposit",
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build deposit activity task", slog.Any("error", err))
			RecordError(span, err)
			return
		}

		if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			log.ErrorContext(ctx, "deposit activity was not queued", slog.Any("error", err))
			RecordError(span, err)
		}
	})
}
