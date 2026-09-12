package events

import (
	"context"
	"fmt"
	"github/marveldo/eda-monolith/internal/queues"
	"log/slog"
)

func RegisterActivityListeners(bus *EventBus, worker queues.Worker[queues.ActivityPayload]) {
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

        task, err := worker.GenerateNewTask(queues.UpdateUserActivity, queues.ActivityPayload{
			UserID: event.UserID,
			Action: "Logged Into Account",
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