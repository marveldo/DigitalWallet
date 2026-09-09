package queues

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github/marveldo/eda-monolith/internal/repository"

	"github.com/hibiken/asynq"
)

// TaskTypeEmailSend is the task's type name, the key RegisterHandlers maps to
// a handler. Changing it orphans any task already sitting in Redis.
const TaskTypeEmailSend = "email:send"

// EmailPayload is what gets marshalled into the task. Keep it small and
// serialisable — it crosses a process boundary, so pass an id and re-read the
// record in the handler rather than embedding a whole entity.
type EmailPayload struct {
	To       string            `json:"to"`
	Subject  string            `json:"subject"`
	Template string            `json:"template"`
	Data     map[string]string `json:"data,omitempty"`
}

// EmailSender is the actual delivery mechanism. Swapping in SES, Resend,
// Postmark or similar is a matter of providing a different implementation to
// NewAsyncWorker — nothing else in the queue changes.
type EmailSender interface {
	Send(ctx context.Context, payload EmailPayload) error
}

// LogEmailSender is the default: it logs what it would have sent. It keeps the
// queue fully wired end to end before a real provider is configured.
type LogEmailSender struct {
	Logger *slog.Logger
}

func (s *LogEmailSender) Send(ctx context.Context, payload EmailPayload) error {
	s.Logger.WarnContext(ctx, "no email provider configured, email not sent",
		slog.String("to", payload.To),
		slog.String("subject", payload.Subject),
		slog.String("template", payload.Template),
	)
	return nil
}

type EmailWorker struct {
	*asynq.Client
	*repository.Repository
	Logger *slog.Logger
	Queue  string
	Sender EmailSender
}

type EmailWorkerConfig struct {
	Client     *asynq.Client
	Repository *repository.Repository
	Logger     *slog.Logger
	Queue      string
	Sender     EmailSender
}

func NewEmailWorker(cfg *EmailWorkerConfig) *EmailWorker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("worker", "email"))

	sender := cfg.Sender
	if sender == nil {
		sender = &LogEmailSender{Logger: logger}
	}
	return &EmailWorker{
		Client:     cfg.Client,
		Repository: cfg.Repository,
		Logger:     logger,
		Queue:      cfg.Queue,
		Sender:     sender,
	}
}

// EnqueueEmail is the typed entry point callers should use: it marshals the
// payload and puts the task on the queue this worker's server consumes.
func (w *EmailWorker) EnqueueEmail(ctx context.Context, payload EmailPayload) (*asynq.TaskInfo, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling email payload: %w", err)
	}
	task := w.GenerateNewTask(TaskTypeEmailSend, body)
	info := w.EnqueueWithContext(task, ctx)
	if info == nil {
		return nil, fmt.Errorf("could not enqueue %s task", TaskTypeEmailSend)
	}
	return info, nil
}

// EnqueueWithContext pushes a task onto the queue. It returns nil when the
// enqueue fails: a queued email is not worth taking the process down for, so
// the failure is logged and the caller decides what to do with a nil info.
//
// asynq.Queue is required here — without it the task lands on "default" while
// the server only consumes the configured namespace, and it would never run.
func (w *EmailWorker) EnqueueWithContext(task *asynq.Task, ctx context.Context) *asynq.TaskInfo {
	opts := []asynq.Option{asynq.MaxRetry(3)}
	if w.Queue != "" {
		opts = append(opts, asynq.Queue(w.Queue))
	}

	info, err := w.Client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		w.Logger.ErrorContext(ctx, "failed to push task to redis",
			slog.String("task_type", task.Type()),
			slog.Any("error", err),
		)
		return nil
	}
	w.Logger.InfoContext(ctx, "task enqueued",
		slog.String("task_id", info.ID),
		slog.String("task_type", task.Type()),
		slog.String("queue", info.Queue),
	)
	return info
}

func (w *EmailWorker) GenerateNewTask(name string, payload []byte) *asynq.Task {
	return asynq.NewTask(name, payload)
}

// HandleEmailSend processes one task. A returned error tells asynq to retry
// (up to MaxRetry, with backoff); asynq.SkipRetry marks the task as failed
// immediately, which is what a payload that will never parse deserves.
func (w *EmailWorker) HandleEmailSend(ctx context.Context, task *asynq.Task) error {
	var payload EmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.Logger.ErrorContext(ctx, "unprocessable email payload", slog.Any("error", err))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	log := w.Logger.With(
		slog.String("task_type", task.Type()),
		slog.String("to", payload.To),
		slog.String("template", payload.Template),
	)

	if err := w.Sender.Send(ctx, payload); err != nil {
		log.ErrorContext(ctx, "failed sending email, will retry", slog.Any("error", err))
		return err
	}
	log.InfoContext(ctx, "email sent")
	return nil
}
