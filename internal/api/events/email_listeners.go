package events

import (
	"context"
	"log/slog"

	"github/marveldo/eda-monolith/internal/queues"
)

// RegisterEmailListeners is the bridge between the bus and the queue: it is
// the only place that knows a user signing up should result in an email.
//
// The generic parameter is resolved here, where the payload type is obvious,
// which is why EventBus itself needs no type parameter. Every domain adds its
// own Register*Listeners naming its own queues.Worker[T].
func RegisterEmailListeners(bus *EventBus, worker queues.Worker[queues.EmailPayload]) {
	bus.Subscribe(EventUserCreated, func(ctx context.Context, payload any) {
		event, ok := payload.(UserCreatedPayload)
		if !ok {
			bus.Logger.ErrorContext(ctx, "unexpected payload type for event",
				slog.String("event", EventUserCreated),
			)
			return
		}

		log := bus.Logger.With(
			slog.String("event", EventUserCreated),
			slog.String("user_id", event.UserID),
		)

		task, err := worker.GenerateNewTask(queues.TaskTypeEmailSend, queues.EmailPayload{
			To:       event.Email,
			Subject:  "Welcome",
			Template: "welcome",
			Data: map[string]string{
				"first_name": event.FirstName,
				"last_name":  event.LastName,
			},
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build welcome email task", slog.Any("error", err))
			return
		}

		// The listener only enqueues. Sending happens out of process in
		// EmailWorker.HandleEmailSend, so signup latency stays independent of
		// the mail provider.
		if info := worker.EnqueueWithContext(task, ctx); info == nil {
			log.ErrorContext(ctx, "welcome email was not queued")
		}
	})
}
