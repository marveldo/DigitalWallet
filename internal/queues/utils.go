package queues

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

func GenerateNewTask[T any](name string, payload T) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling email payload: %w", err)
	}
	return asynq.NewTask(name, body), nil
}

func EnqueueWithContext(task *asynq.Task, ctx context.Context, queue string, client *asynq.Client, logger *slog.Logger) (*asynq.TaskInfo, error) {
	opts := []asynq.Option{asynq.MaxRetry(3)}
	if queue != "" {
		opts = append(opts, asynq.Queue(queue))
	}

	info, err := client.EnqueueContext(ctx, task, opts...)
	if err != nil {
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
