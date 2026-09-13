package queues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/internal/templates"
	"github/marveldo/eda-monolith/shared"

	"github.com/hibiken/asynq"
)

const TaskTypePaymentVerify = "payment:verify"

type PaymentPayload struct {
	Reference string `json:"reference"`
	Provider  string `json:"provider"`
	Event     string `json:"event,omitempty"`
	Status    string `json:"status,omitempty"`
}

type PaymentWorker struct {
	*asynq.Client
	*repository.Repository
	Logger   *slog.Logger
	Queue    string
	Provider shared.PaymentProvider
	Email    *EmailWorker
	Activity *ActivityWorker
}

type PaymentWorkerConfig struct {
	Client     *asynq.Client
	Repository *repository.Repository
	Logger     *slog.Logger
	Queue      string
	Provider   shared.PaymentProvider
	Email      *EmailWorker
	Activity   *ActivityWorker
}

func NewPaymentWorker(cfg *PaymentWorkerConfig) *PaymentWorker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("worker", "payment"))

	return &PaymentWorker{
		Client:     cfg.Client,
		Repository: cfg.Repository,
		Logger:     logger,
		Queue:      cfg.Queue,
		Provider:   cfg.Provider,
		Email:      cfg.Email,
		Activity:   cfg.Activity,
	}
}

func (p *PaymentWorker) EnqueueWithContext(task *asynq.Task, ctx context.Context) (*asynq.TaskInfo, error) {
	return EnqueueWithContext(task, ctx, p.Queue, p.Client, p.Logger)
}

func (p *PaymentWorker) GenerateNewTask(name string, payload PaymentPayload) (*asynq.Task, error) {
	return GenerateNewTask[PaymentPayload](name, payload)
}

func (p *PaymentWorker) HandleVerifyPayment(ctx context.Context, task *asynq.Task) error {
	var payload PaymentPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		p.Logger.ErrorContext(ctx, "unprocessable payment payload", slog.Any("error", err))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	log := p.Logger.With(
		slog.String("task_type", task.Type()),
		slog.String("reference", payload.Reference),
		slog.String("provider", payload.Provider),
	)

	if p.Provider == nil {
		log.ErrorContext(ctx, "no payment provider configured, cannot verify")
		return fmt.Errorf("%w: no payment provider configured", asynq.SkipRetry)
	}

	repoCtx := p.Repository.NewRepoCtx(repository.RepoctxConfig{Context: ctx, Logger: log})

	verification, err := p.Provider.Verify(ctx, payload.Reference)
	if err != nil {
		if errors.Is(err, shared.ErrReferenceNotFound) {
			log.WarnContext(ctx, "provider does not know this reference")
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		log.ErrorContext(ctx, "could not verify payment, will retry", slog.Any("error", err))
		return err
	}

	log = log.With(slog.String("provider_status", string(verification.Status)))

	switch verification.Status {
	case shared.PaymentSuccess:
		return p.settle(ctx, repoCtx, log, verification)
	case shared.PaymentFailed, shared.PaymentAbandoned:
		if _, err := p.Repository.MarkIntentFailed(repoCtx, payload.Reference); err != nil {
			if errors.Is(err, repository.ErrIntentNotPending) {
				log.InfoContext(ctx, "intent already resolved, nothing to fail")
				return nil
			}
			log.ErrorContext(ctx, "could not mark intent failed", slog.Any("error", err))
			return err
		}
		log.InfoContext(ctx, "payment marked failed")
		return nil
	default:
		log.InfoContext(ctx, "payment still pending at the provider, leaving the intent open")
		return nil
	}
}

func (p *PaymentWorker) settle(ctx context.Context, repoCtx *repository.RepoCtx, log *slog.Logger, verification *shared.PaymentVerification) error {
	intent, err := p.Repository.SettleIntent(repoCtx, &repository.SettleIntentParam{
		Reference:   verification.Reference,
		Amount:      verification.Amount,
		Description: fmt.Sprintf("Deposit via %s", p.Provider.Name()),
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrIntentNotPending):
			log.InfoContext(ctx, "intent already settled, skipping")
			return nil
		case errors.Is(err, repository.ErrAmountMismatch):
			log.ErrorContext(ctx, "provider settled a different amount than the intent",
				slog.Int64("verified_minor", verification.Amount.Minor()))
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		case errors.Is(err, repository.ErrNotFound):
			log.ErrorContext(ctx, "no intent exists for this reference")
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		default:
			log.ErrorContext(ctx, "could not settle payment, will retry", slog.Any("error", err))
			return err
		}
	}

	log.InfoContext(ctx, "payment settled and wallet credited",
		slog.Int64("amount_minor", intent.Amount.Minor()),
		slog.String("wallet_id", intent.WalletID),
	)

	p.notify(ctx, repoCtx, log, intent)
	return nil
}

func (p *PaymentWorker) notify(ctx context.Context, repoCtx *repository.RepoCtx, log *slog.Logger, intent *repository.TransactionIntent) {
	if p.Activity != nil {
		task, err := p.Activity.GenerateNewTask(UpdateUserActivity, ActivityPayload{
			UserID: intent.UserID,
			Action: fmt.Sprintf("Deposited %s into wallet", intent.Amount.String()),
		})
		if err != nil {
			log.ErrorContext(ctx, "could not build deposit activity task", slog.Any("error", err))
		} else if _, err := p.Activity.EnqueueWithContext(task, ctx); err != nil {
			log.ErrorContext(ctx, "deposit activity was not queued", slog.Any("error", err))
		}
	}

	if p.Email == nil {
		return
	}

	user, err := p.Repository.GetUserByID(repoCtx, intent.UserID)
	if err != nil {
		log.ErrorContext(ctx, "could not load user for the deposit receipt", slog.Any("error", err))
		return
	}

	task, err := p.Email.GenerateNewTask(TaskTypeEmailSend, EmailPayload{
		To:        []string{user.Email},
		Subject:   "Your DigiWallet Deposit Was Successful",
		Template:  templates.EmailDepositSuccess,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Data: map[string]string{
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"amount":     intent.Amount.String(),
			"reference":  intent.ID,
		},
	})
	if err != nil {
		log.ErrorContext(ctx, "could not build deposit receipt task", slog.Any("error", err))
		return
	}
	if _, err := p.Email.EnqueueWithContext(task, ctx); err != nil {
		log.ErrorContext(ctx, "deposit receipt was not queued", slog.Any("error", err))
	}
}
