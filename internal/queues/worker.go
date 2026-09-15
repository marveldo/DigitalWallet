package queues

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github/marveldo/eda-monolith/internal/ledger"

	"github/marveldo/eda-monolith/config"
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/shared"

	"github.com/hibiken/asynq"
)

type Worker[T any] interface {
	EnqueueWithContext(task *asynq.Task, ctx context.Context) (*asynq.TaskInfo, error)
	GenerateNewTask(name string, payload T) (*asynq.Task, error)
}

type AsynqWorkerStruct struct {
	srv    *asynq.Server
	mux    *asynq.ServeMux
	Logger *slog.Logger
	Queue  string
	*EmailWorker
	*ActivityWorker
	*PaymentWorker
}

type AsynqWorkerConfig struct {
	Config     *config.AsynqBackgroundWorker
	Repository *repository.Repository
	Logger     *slog.Logger
	Sender     EmailSender
	Provider   shared.PaymentProvider
	Ledger     *ledger.Ledger
	// PollInterval and PollMaxRetry drive the payment verify poller: it checks
	// the provider every PollInterval, PollMaxRetry times, before giving up.
	PollInterval time.Duration
	PollMaxRetry int
}

func NewAsyncWorker(cfg *AsynqWorkerConfig) (*AsynqWorkerStruct, error) {
	workerCfg := cfg.Config
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("layer", "queues"))

	queueName := workerCfg.RedisNamespace
	if queueName == "" {
		queueName = "default"
	}
	concurrency := workerCfg.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	connOpts, err := RedisConnOpt(workerCfg)
	if err != nil {
		return nil, err
	}
	client := NewAsyncClient(connOpts)
	repository := cfg.Repository
	emailWorker := NewEmailWorker(&EmailWorkerConfig{
		Client:     client,
		Repository: repository,
		Logger:     logger,
		Queue:      queueName,
		Sender:     cfg.Sender,
	})

	activityWorker := NewActivityWorker(&ActivityWorkerConfig{
		Client:     client,
		Repository: repository,
		Logger:     logger,
		Queue:      queueName,
	})

	paymentWorker := NewPaymentWorker(&PaymentWorkerConfig{
		Client:       client,
		Repository:   repository,
		Logger:       logger,
		Queue:        queueName,
		Provider:     cfg.Provider,
		Ledger:       cfg.Ledger,
		Email:        emailWorker,
		Activity:     activityWorker,
		PollInterval: cfg.PollInterval,
		PollMaxRetry: cfg.PollMaxRetry,
	})

	asynccfg := asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{queueName: 1},
		Logger:      NewAsynqSlogAdapter(logger),
		RetryDelayFunc: RetryDelay(map[string]time.Duration{
			TaskTypePaymentPoll: paymentWorker.PollInterval,
		}),
		IsFailure: IsFailure,
	}

	w := &AsynqWorkerStruct{
		srv:            asynq.NewServer(connOpts, asynccfg),
		mux:            asynq.NewServeMux(),
		Logger:         logger,
		Queue:          queueName,
		EmailWorker:    emailWorker,
		ActivityWorker: activityWorker,
		PaymentWorker:  paymentWorker,
	}
	w.RegisterHandlers()
	return w, nil
}

// RedisConnOpt turns the configured URL into asynq's connection options.
// ParseRedisURI understands redis://, rediss://, redis-socket:// and
// redis-sentinel://, and picks the db number out of the path — so the URL is
// never hand-split here. The explicit username/password/db settings win over
// anything embedded in the URL.
func RedisConnOpt(cfg *config.AsynqBackgroundWorker) (asynq.RedisConnOpt, error) {
	parsed, err := asynq.ParseRedisURI(cfg.RedisUrl)
	if err != nil {
		return nil, fmt.Errorf("parsing ASYNC_REDIS_URL %q: %w", cfg.RedisUrl, err)
	}

	clientOpt, ok := parsed.(asynq.RedisClientOpt)
	if !ok {
		// A sentinel/cluster URL carries its own auth; return it untouched
		// rather than dropping the fields we cannot set here.
		return parsed, nil
	}
	if cfg.Username != "" {
		clientOpt.Username = cfg.Username
	}
	if cfg.Password != "" {
		clientOpt.Password = cfg.Password
	}
	if cfg.DB != 0 {
		clientOpt.DB = cfg.DB
	}
	return clientOpt, nil
}

func (w *AsynqWorkerStruct) RegisterHandlers() {
	w.mux.HandleFunc(TaskTypeEmailSend, w.EmailWorker.HandleEmailSend)
	w.mux.HandleFunc(UpdateUserActivity, w.ActivityWorker.HandleCreateActivity)

	// Only the payment tasks run behind the middleware.
	w.handlePayment(TaskTypePaymentWebhook, w.PaymentWorker.HandlePaymentWebhook)
	w.handlePayment(TaskTypePaymentPoll, w.PaymentWorker.HandlePaymentPoll,
		OnRetriesExhausted(w.PaymentWorker.HandlePollExhausted),
	)
}

// handlePayment registers a payment handler behind logging, any extra mws, and
// panic recovery innermost, so every middleware sees a panic as a failed attempt.
func (w *AsynqWorkerStruct) handlePayment(pattern string, h asynq.HandlerFunc, mws ...asynq.MiddlewareFunc) {
	chain := append([]asynq.MiddlewareFunc{LoggingMiddleware(w.PaymentWorker.Logger)}, mws...)
	chain = append(chain, RecoverMiddleware(w.PaymentWorker.Logger))
	w.mux.Handle(pattern, Chain(h, chain...))
}

func (w *AsynqWorkerStruct) Start() error {
	if err := w.srv.Ping(); err != nil {
		return fmt.Errorf("redis unreachable: %w", err)
	}
	w.Logger.Info("starting queue worker", slog.String("queue", w.Queue))
	return w.srv.Start(w.mux)
}

func (w *AsynqWorkerStruct) Shutdown() {
	w.Logger.Info("shutting down queue worker", slog.String("queue", w.Queue))
	w.srv.Shutdown()
	if w.EmailWorker != nil && w.EmailWorker.Client != nil {
		if err := w.EmailWorker.Client.Close(); err != nil {
			w.Logger.Error("failed closing queue client", slog.Any("error", err))
		}
	}
}

func NewAsyncClient(cfg asynq.RedisConnOpt) *asynq.Client {
	return asynq.NewClient(cfg)
}

// AsynqSlogAdapter satisfies asynq.Logger so the queue's own output lands in
// the same structured log as everything else.
type AsynqSlogAdapter struct {
	logger *slog.Logger
}

func NewAsynqSlogAdapter(logger *slog.Logger) *AsynqSlogAdapter {
	return &AsynqSlogAdapter{logger: logger.With(slog.String("component", "asynq"))}
}

func (a *AsynqSlogAdapter) Debug(args ...any) { a.logger.Debug(fmt.Sprint(args...)) }
func (a *AsynqSlogAdapter) Info(args ...any)  { a.logger.Info(fmt.Sprint(args...)) }
func (a *AsynqSlogAdapter) Warn(args ...any)  { a.logger.Warn(fmt.Sprint(args...)) }
func (a *AsynqSlogAdapter) Error(args ...any) { a.logger.Error(fmt.Sprint(args...)) }
func (a *AsynqSlogAdapter) Fatal(args ...any) { a.logger.Error(fmt.Sprint(args...)) }
