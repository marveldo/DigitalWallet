package queues

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/hibiken/asynq"
)

// ErrRetryLater marks an expected retry: the work is not done yet, but nothing
// went wrong (a payment still pending at the provider). It is retried like any
// other error, logged quietly and kept out of asynq's failure stats.
var ErrRetryLater = errors.New("not ready, retry later")

// ExhaustedFunc runs once a task has used its last retry and is about to be
// archived.
type ExhaustedFunc func(ctx context.Context, task *asynq.Task, err error)

// Chain wraps h in mws. The first middleware is the outermost, so
// Chain(h, a, b) runs a, then b, then h.
func Chain(h asynq.Handler, mws ...asynq.MiddlewareFunc) asynq.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// IsFinalAttempt reports whether err ends the task: asynq archives it instead
// of scheduling another retry.
func IsFinalAttempt(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, asynq.SkipRetry) {
		return true
	}
	retried, ok := asynq.GetRetryCount(ctx)
	if !ok {
		return false
	}
	maxRetry, ok := asynq.GetMaxRetry(ctx)
	if !ok {
		return false
	}
	return retried >= maxRetry
}

// IsFailure tells asynq which errors count as failures in its stats.
func IsFailure(err error) bool {
	return !errors.Is(err, ErrRetryLater)
}

// RecoverMiddleware turns a panic into an error, so the retry and exhaustion
// middleware still see the attempt fail.
func RecoverMiddleware(logger *slog.Logger) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) (err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.ErrorContext(ctx, "task handler panicked",
						slog.String("task_type", task.Type()),
						slog.Any("panic", r),
						slog.String("stack", string(debug.Stack())),
					)
					err = fmt.Errorf("task handler panicked: %v", r)
				}
			}()
			return next.ProcessTask(ctx, task)
		})
	}
}

// LoggingMiddleware logs every attempt with its retry position and outcome.
func LoggingMiddleware(logger *slog.Logger) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			taskID, _ := asynq.GetTaskID(ctx)
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			log := logger.With(
				slog.String("task_id", taskID),
				slog.String("task_type", task.Type()),
				slog.Int("attempt", retried+1),
				slog.Int("max_attempts", maxRetry+1),
			)

			start := time.Now()
			err := next.ProcessTask(ctx, task)
			log = log.With(slog.Duration("duration", time.Since(start)))

			switch {
			case err == nil:
				log.InfoContext(ctx, "task completed")
			case errors.Is(err, asynq.SkipRetry):
				log.ErrorContext(ctx, "task dropped without retry", slog.Any("error", err))
			case IsFinalAttempt(ctx, err):
				log.ErrorContext(ctx, "task retries exhausted", slog.Any("error", err))
			case errors.Is(err, ErrRetryLater):
				log.InfoContext(ctx, "task not ready, will retry", slog.Any("reason", err))
			default:
				log.WarnContext(ctx, "task failed, will retry", slog.Any("error", err))
			}
			return err
		})
	}
}

// OnRetriesExhausted calls fn when an attempt fails and no retry is left.
// SkipRetry is left to the handler: it chose to stop, so it already handled
// the outcome.
func OnRetriesExhausted(fn ExhaustedFunc) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			err := next.ProcessTask(ctx, task)
			if err != nil && !errors.Is(err, asynq.SkipRetry) && IsFinalAttempt(ctx, err) {
				fn(ctx, task, err)
			}
			return err
		})
	}
}

// RetryDelay uses a fixed delay for the listed task types and asynq's
// exponential backoff for everything else.
func RetryDelay(fixed map[string]time.Duration) asynq.RetryDelayFunc {
	return func(n int, err error, task *asynq.Task) time.Duration {
		if delay, ok := fixed[task.Type()]; ok && delay > 0 {
			return delay
		}
		return asynq.DefaultRetryDelayFunc(n, err, task)
	}
}
