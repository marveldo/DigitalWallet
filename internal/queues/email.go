package queues

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/internal/templates"

	"github.com/hibiken/asynq"
)

const TaskTypeEmailSend = "email:send"

type EmailPayload struct {
	To        []string          `json:"to"`
	Subject   string            `json:"subject"`
	Template  string            `json:"template"`
	OTP       string            `json:"otp,omitempty"`
	FirstName string            `json:"first_name,omitempty"`
	LastName  string            `json:"last_name,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

func (p EmailPayload) Recipient() string {
	if len(p.To) == 0 {
		return ""
	}
	return strings.Join(p.To, ", ")
}

type EmailSender interface {
	Send(ctx context.Context, payload EmailPayload, body string) error
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

	return &EmailWorker{
		Client:     cfg.Client,
		Repository: cfg.Repository,
		Logger:     logger,
		Queue:      cfg.Queue,
		Sender:     sender,
	}
}

func (w *EmailWorker) EnqueueWithContext(task *asynq.Task, ctx context.Context) (*asynq.TaskInfo, error) {
	return EnqueueWithContext(task, ctx, w.Queue, w.Client, w.Logger)
}

func (w *EmailWorker) GenerateNewTask(name string, payload EmailPayload) (*asynq.Task, error) {
	return GenerateNewTask[EmailPayload](name, payload)
}

func (w *EmailWorker) HandleEmailSend(ctx context.Context, task *asynq.Task) error {
	var payload EmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.Logger.ErrorContext(ctx, "unprocessable email payload", slog.Any("error", err))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	log := w.Logger.With(
		slog.String("task_type", task.Type()),
		slog.Any("to", payload.To),
		slog.String("template", payload.Template),
	)

	body, err := templates.RenderEmail(payload.Template, payload)
	if err != nil {
		log.ErrorContext(ctx, "could not render email template", slog.Any("error", err))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	if err := w.Sender.Send(ctx, payload, body); err != nil {
		log.ErrorContext(ctx, "failed sending email, will retry", slog.Any("error", err))
		return err
	}
	log.InfoContext(ctx, "email sent")
	return nil
}
