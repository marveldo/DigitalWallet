package queues

import (
	"context"
	"encoding/json"
	"fmt"
	"github/marveldo/eda-monolith/internal/repository"
	"log/slog"

	"github.com/hibiken/asynq"
)

const UpdateUserActivity = "user:activity_update"

type ActivityPayload struct {
	UserID string    `json:"user_id"`
	Action string    `json:"action"`
}

type ActivityWorker struct {
	*asynq.Client
	*repository.Repository
	Logger *slog.Logger
	Queue  string
}
type ActivityWorkerConfig struct {
	*asynq.Client
	*repository.Repository
	Logger *slog.Logger
	Queue  string
}
func NewActivityWorker(cfg *ActivityWorkerConfig) *ActivityWorker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("worker", "email"))

    
	return &ActivityWorker{
		Client:     cfg.Client,
		Repository: cfg.Repository,
		Logger:     logger,
		Queue:      cfg.Queue,
	}
}

func (a *ActivityWorker) EnqueueWithContext(task *asynq.Task, ctx context.Context) (*asynq.TaskInfo, error) {
	return EnqueueWithContext(task, ctx, a.Queue, a.Client, a.Logger)
}

func (a *ActivityWorker) GenerateNewTask(name string, payload ActivityPayload) (*asynq.Task, error) {
	return  GenerateNewTask[ActivityPayload](name , payload)
}

func (a *ActivityWorker) HandleCreateActivity(ctx context.Context, task *asynq.Task) error {
	var activityPayload ActivityPayload

	if err := json.Unmarshal(task.Payload(), &activityPayload) ; err != nil {
		a.Logger.ErrorContext(ctx, "unprocessable email payload", slog.Any("error", err))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
    
	log := a.Logger.With(
		slog.String("task_type", task.Type()),
		slog.Any("to", activityPayload.UserID),
		slog.String("template", activityPayload.Action),
	)
   
	repo_ctx := a.Repository.NewRepoCtx(repository.RepoctxConfig{Context: ctx , Logger: log})

	_ , err := a.Repository.ActivityRepository.CreateActivity(repo_ctx , activityPayload.UserID , activityPayload.Action)
	if err != nil {
       	log.ErrorContext(ctx, "could not Create Activity", slog.Any("error", err))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	log.InfoContext(ctx, "User Activity Created")
    
	return nil

}