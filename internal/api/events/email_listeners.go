package events

import (
	"context"
	"fmt"
	"log/slog"

	"github/marveldo/eda-monolith/internal/otp"
	"github/marveldo/eda-monolith/internal/queues"
	"github/marveldo/eda-monolith/internal/templates"
)

func RegisterEmailListeners(bus *EventBus, worker queues.Worker[queues.EmailPayload], otpStore *otp.Store) {
	bus.Subscribe(EventUserCreated, func(ctx context.Context, payload any) {
		ctx, span := bus.StartSpan(ctx, "listener.email.welcome")
		defer span.End()

		event, ok := payload.(UserCreatedPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventUserCreated))
			bus.Logger.ErrorContext(ctx, "unexpected payload type for event",
				slog.String("event", EventUserCreated),
			)
			return
		}

		log := bus.Logger.With(
			slog.String("event", EventUserCreated),
			slog.String("user_id", event.UserID),
		)

		code, err := otpStore.Generate(ctx, event.Email)
		if err != nil {
			log.ErrorContext(ctx, "could not issue otp", slog.Any("error", err))
			RecordError(span, err)
			return
		}

		task, err := worker.GenerateNewTask(queues.TaskTypeEmailSend, queues.EmailPayload{
			To:        []string{event.Email},
			Subject:   "Welcome to DigiWallet",
			Template:  templates.EmailWelcome,
			OTP:       code,
			FirstName: event.FirstName,
			LastName:  event.LastName,
			Data: map[string]string{
				"first_name": event.FirstName,
				"last_name":  event.LastName,
			},
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build welcome email task", slog.Any("error", err))
			RecordError(span, err)
			return
		}

		if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			log.ErrorContext(ctx, "welcome email was not queued", slog.Any("error", err))
			RecordError(span, err)
		}
	})

	bus.Subscribe(EventUserLoggedIn , func(ctx context.Context, payload any) {
		ctx , span := bus.StartSpan(ctx , "listener.email.loginalert")
		defer span.End()
		event , ok := payload.(UserLoggedInPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventUserLoggedIn))
			bus.Logger.ErrorContext(ctx, "unexpected payload type for event",
				slog.String("event", EventUserLoggedIn),
			)
			return
		}
		log := bus.Logger.With(
			slog.String("event", EventUserLoggedIn),
			slog.String("user_id", event.UserID),
		)

        task, err := worker.GenerateNewTask(queues.TaskTypeEmailSend, queues.EmailPayload{
			To:        []string{event.Email},
			Subject:   "Logged Into DigiWallet Account",
			Template:  templates.EmailLoginAlert,
			FirstName: event.FirstName,
			LastName:  event.LastName,
			Data: map[string]string{
				"first_name": event.FirstName,
				"last_name":  event.LastName,
			},
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build Log in email task", slog.Any("error", err))
			RecordError(span, err)
			return
		}
        if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			log.ErrorContext(ctx, "welcome Login Email was not queued", slog.Any("error", err))
			RecordError(span, err)
		}
})

}
