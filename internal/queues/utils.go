package queues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

func GenerateNewTask[T any](name string, payload T, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling %s payload: %w", name, err)
	}
	// MaxRetry(3) is the default; opts are applied after it, so they win.
	return asynq.NewTask(name, body, append([]asynq.Option{asynq.MaxRetry(3)}, opts...)...), nil
}

func EnqueueWithContext(task *asynq.Task, ctx context.Context, queue string, client *asynq.Client, logger *slog.Logger) (*asynq.TaskInfo, error) {
	var opts []asynq.Option
	if queue != "" {
		opts = append(opts, asynq.Queue(queue))
	}

	info, err := client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			logger.DebugContext(ctx, "task already queued", slog.String("task_type", task.Type()))
			return nil, nil
		}
		logger.ErrorContext(ctx, "failed to push task to redis",
			slog.String("task_type", task.Type()),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("enqueue %s: %w", task.Type(), err)
	}
	logger.InfoContext(ctx, "task enqueued",
		slog.String("task_id", info.ID),
		slog.String("task_type", task.Type()),
		slog.String("queue", info.Queue),
	)
	return info, nil
}
