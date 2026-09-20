package events

import (
	"context"
	"fmt"
	"log/slog"

	"github/marveldo/eda-monolith/internal/queues"
)

func RegisterTransferListeners(bus *EventBus, worker queues.Worker[queues.TransferPayload]) {
	bus.Subscribe(EventTransferInitiated, func(ctx context.Context, payload any) {
		ctx, span := bus.StartSpan(ctx, "listener.transfer.execute")
		defer span.End()

		event, ok := payload.(TransferInitiatedPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventTransferInitiated))
			bus.Logger.ErrorContext(ctx, "unexpected payload type for event",
				slog.String("event", EventTransferInitiated),
			)
			return
		}

		log := bus.Logger.With(
			slog.String("event", EventTransferInitiated),
			slog.String("transfer_id", event.TransferID),
			slog.String("user_id", event.UserID),
		)

		task, err := worker.GenerateNewTask(queues.TaskTypeTransferExecute, queues.TransferPayload{
			TransferID: event.TransferID,
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build transfer task", slog.Any("error", err))
			RecordError(span, err)
			return
		}
		if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			log.ErrorContext(ctx, "transfer was not queued, it stays pending", slog.Any("error", err))
			RecordError(span, err)
		}
	})
}
func RegisterTransferActivityListeners(bus *EventBus, worker queues.Worker[queues.ActivityPayload]) {
	bus.Subscribe(EventTransferInitiated, func(ctx context.Context, payload any) {
		ctx, span := bus.StartSpan(ctx, "listener.transfer.activity")
		defer span.End()

		event, ok := payload.(TransferInitiatedPayload)
		if !ok {
			RecordError(span, fmt.Errorf("unexpected payload type %T for %s", payload, EventTransferInitiated))
			return
		}

		task, err := worker.GenerateNewTask(queues.UpdateUserActivity, queues.ActivityPayload{
			UserID: event.UserID,
			Action: "Started a transfer",
		})
		if err != nil {
			bus.Logger.ErrorContext(ctx, "could not build transfer activity task", slog.Any("error", err))
			RecordError(span, err)
			return
		}
		if _, err := worker.EnqueueWithContext(task, ctx); err != nil {
			bus.Logger.ErrorContext(ctx, "transfer activity was not queued", slog.Any("error", err))
			RecordError(span, err)
		}
	})
}
