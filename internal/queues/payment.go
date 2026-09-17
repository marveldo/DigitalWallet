package queues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github/marveldo/eda-monolith/internal/ledger"
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/internal/templates"
	"github/marveldo/eda-monolith/shared"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskTypePaymentWebhook settles a deposit from a signed provider webhook.
	TaskTypePaymentWebhook = "payment:webhook"
	// TaskTypePaymentPoll asks the provider for a deposit's status in case the
	// webhook never arrives. It retries until the payment resolves or the
	// retries run out, and then the intent is marked failed.
	TaskTypePaymentPoll = "payment:poll"

	DefaultPollInterval = time.Minute
	DefaultPollMaxRetry = 30
	PaymentWebhookRetry = 10

	intentStatusPending = "PENDING"
	intentStatusSuccess = "SUCCESS"
	intentStatusFailed  = "FAILED"
	intentStatusRefund  = "REFUNDING"
	walletStatusActive  = "ACTIVE"
)

var (
	ErrAmountMismatch   = errors.New("settled amount does not match the intent")
	ErrCurrencyMismatch = errors.New("settled currency does not match the wallet")
	ErrWalletNotActive  = errors.New("wallet is not active")
)

type PaymentPayload struct {
	Reference   string `json:"reference"`
	Provider    string `json:"provider"`
	Event       string `json:"event,omitempty"`
	Status      string `json:"status,omitempty"`
	AmountMinor int64  `json:"amount_minor,omitempty"`
	Currency    string `json:"currency,omitempty"`
}

type PaymentWorker struct {
	*asynq.Client
	Inspector *asynq.Inspector
	*repository.Repository
	Logger       *slog.Logger
	Queue        string
	Provider     shared.PaymentProvider
	Ledger       *ledger.Ledger
	Email        *EmailWorker
	Activity     *ActivityWorker
	PollInterval time.Duration
	PollMaxRetry int
}

type PaymentWorkerConfig struct {
	Client       *asynq.Client
	Inspector    *asynq.Inspector
	Repository   *repository.Repository
	Logger       *slog.Logger
	Queue        string
	Provider     shared.PaymentProvider
	Ledger       *ledger.Ledger
	Email        *EmailWorker
	Activity     *ActivityWorker
	PollInterval time.Duration
	PollMaxRetry int
}

func NewPaymentWorker(cfg *PaymentWorkerConfig) *PaymentWorker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("worker", "payment"))

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	pollMaxRetry := cfg.PollMaxRetry
	if pollMaxRetry <= 0 {
		pollMaxRetry = DefaultPollMaxRetry
	}

	return &PaymentWorker{
		Client:       cfg.Client,
		Inspector:    cfg.Inspector,
		Repository:   cfg.Repository,
		Logger:       logger,
		Queue:        cfg.Queue,
		Provider:     cfg.Provider,
		Ledger:       cfg.Ledger,
		Email:        cfg.Email,
		Activity:     cfg.Activity,
		PollInterval: pollInterval,
		PollMaxRetry: pollMaxRetry,
	}
}

func (p *PaymentWorker) EnqueueWithContext(task *asynq.Task, ctx context.Context) (*asynq.TaskInfo, error) {
	return EnqueueWithContext(task, ctx, p.Queue, p.Client, p.Logger)
}

func PollTaskID(reference string) string {
	return TaskTypePaymentPoll + ":" + reference
}

// GenerateNewTask attaches each payment task's retry policy, so callers only
// pick the task type.
func (p *PaymentWorker) GenerateNewTask(name string, payload PaymentPayload) (*asynq.Task, error) {
	switch name {
	case TaskTypePaymentPoll:
		return GenerateNewTask(name, payload,
			asynq.MaxRetry(p.PollMaxRetry),
			asynq.ProcessIn(p.PollInterval),
			// One poller per deposit, however often it is enqueued.
			asynq.TaskID(PollTaskID(payload.Reference)),
		)
	case TaskTypePaymentWebhook:
		return GenerateNewTask(name, payload, asynq.MaxRetry(PaymentWebhookRetry))
	default:
		return GenerateNewTask(name, payload)
	}
}

func (p *PaymentWorker) decode(task *asynq.Task) (PaymentPayload, *slog.Logger, error) {
	var payload PaymentPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return payload, p.Logger, fmt.Errorf("%w: unprocessable payment payload: %v", asynq.SkipRetry, err)
	}
	log := p.Logger.With(
		slog.String("task_type", task.Type()),
		slog.String("reference", payload.Reference),
		slog.String("provider", payload.Provider),
	)
	return payload, log, nil
}

func (p *PaymentWorker) repoCtx(ctx context.Context, log *slog.Logger) *repository.RepoCtx {
	return p.Repository.NewRepoCtx(repository.RepoctxConfig{Context: ctx, Logger: log})
}

// HandlePaymentWebhook acts on the provider's signed webhook: a success goes
// straight to the ledger, a failure closes the intent.
func (p *PaymentWorker) HandlePaymentWebhook(ctx context.Context, task *asynq.Task) error {
	payload, log, err := p.decode(task)
	if err != nil {
		return err
	}

	switch shared.PaymentStatus(payload.Status) {
	case shared.PaymentSuccess:
		err = p.settle(ctx, log, payload.Reference, shared.NewMoneyFromMinor(payload.AmountMinor), payload.Currency)
	case shared.PaymentFailed:
		err = p.fail(ctx, log, payload.Reference)
	default:
		log.InfoContext(ctx, "webhook does not resolve the payment, leaving it to the poller",
			slog.String("status", payload.Status))
		return nil
	}
	if err != nil {
		return err
	}
	p.cancelPoll(ctx, log, payload.Reference)
	return nil
}

func (p *PaymentWorker) cancelPoll(ctx context.Context, log *slog.Logger, reference string) {
	if p.Inspector == nil {
		return
	}
	err := p.Inspector.DeleteTask(p.Queue, PollTaskID(reference))
	switch {
	case err == nil:
		log.InfoContext(ctx, "deposit resolved by webhook, poll task removed")
	case errors.Is(err, asynq.ErrTaskNotFound), errors.Is(err, asynq.ErrQueueNotFound):
		// Already finished, or never enqueued.
	default:
		// Most often the poll is running right now; it will stop on its own.
		log.InfoContext(ctx, "could not remove poll task, it will stop on its next run", slog.Any("error", err))
	}
}

func (p *PaymentWorker) HandlePaymentPoll(ctx context.Context, task *asynq.Task) error {
	payload, log, err := p.decode(task)
	if err != nil {
		return err
	}
	if p.Provider == nil {
		return fmt.Errorf("%w: no payment provider configured", asynq.SkipRetry)
	}

	intent, err := p.Repository.GetIntentByReference(p.repoCtx(ctx, log), payload.Reference)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: no intent for this reference: %v", asynq.SkipRetry, err)
		}
		return err
	}
	if intent.Status != intentStatusPending {
		log.InfoContext(ctx, "intent already resolved, polling stopped", slog.String("status", intent.Status))
		return nil
	}

	verification, err := p.Provider.Verify(ctx, payload.Reference)
	if err != nil {
		if errors.Is(err, shared.ErrReferenceNotFound) {
			return fmt.Errorf("%w: provider does not know this reference yet", ErrRetryLater)
		}
		return err
	}

	switch verification.Status {
	case shared.PaymentSuccess:
		return p.settle(ctx, log, payload.Reference, verification.Amount, verification.Currency)
	case shared.PaymentFailed:
		return p.fail(ctx, log, payload.Reference)
	default:
		// Paystack reports an unpaid checkout as abandoned while the user can
		// still pay it, so abandoned keeps polling like pending.
		return fmt.Errorf("%w: provider status %s", ErrRetryLater, verification.Status)
	}
}

func (p *PaymentWorker) HandlePollExhausted(ctx context.Context, task *asynq.Task, cause error) {
	payload, log, err := p.decode(task)
	if err != nil {
		log.ErrorContext(ctx, "could not read exhausted payment task", slog.Any("error", err))
		return
	}
	log = log.With(slog.Any("cause", cause))
	if err := p.fail(ctx, log, payload.Reference); err != nil {
		log.ErrorContext(ctx, "could not mark unresolved deposit failed", slog.Any("error", err))
		return
	}
	log.WarnContext(ctx, "deposit never resolved at the provider, marked failed")
}

func (p *PaymentWorker) settle(ctx context.Context, log *slog.Logger, reference string, amount shared.Money, currency string) error {
	if p.Ledger == nil {
		return fmt.Errorf("%w: no ledger configured", asynq.SkipRetry)
	}
	repoCtx := p.repoCtx(ctx, log)

	intent, err := p.Repository.GetIntentByReference(repoCtx, reference)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: no intent for this reference: %v", asynq.SkipRetry, err)
		}
		return err
	}
	switch intent.Status {
	case intentStatusPending:
	case intentStatusFailed, intentStatusRefund:
		// The payment went through after the intent was given up on.
		return p.refundLatePayment(ctx, log, intent)
	default:
		log.InfoContext(ctx, "intent already resolved, skipping", slog.String("status", intent.Status))
		return nil
	}
	if amount != intent.Amount {
		log.ErrorContext(ctx, "provider settled a different amount than the intent",
			slog.Int64("intent_minor", intent.Amount.Minor()),
			slog.Int64("settled_minor", amount.Minor()))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, ErrAmountMismatch)
	}

	wallet, err := p.Repository.GetWalletByID(repoCtx, intent.WalletID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: wallet for this intent is gone: %v", asynq.SkipRetry, err)
		}
		return err
	}
	if currency != "" && !strings.EqualFold(currency, wallet.Currency) {
		log.ErrorContext(ctx, "provider settled in a different currency than the wallet",
			slog.String("wallet_currency", wallet.Currency),
			slog.String("settled_currency", currency))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, ErrCurrencyMismatch)
	}
	if wallet.Status != walletStatusActive {
		log.ErrorContext(ctx, "payment received for a wallet that is not active, needs manual review",
			slog.String("wallet_status", wallet.Status))
		return fmt.Errorf("%w: %v", asynq.SkipRetry, ErrWalletNotActive)
	}

	intentID, err := uuid.Parse(intent.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	walletID, err := uuid.Parse(wallet.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	err = p.Ledger.Deposit(ctx, ledger.DepositParam{
		TransferID: intentID,
		WalletID:   walletID,
		Currency:   wallet.Currency,
		Amount:     intent.Amount,
	})
	if err != nil {
		if errors.Is(err, ledger.ErrRejected) {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
		}
		return err
	}

	settled, err := p.Repository.MarkIntentSucceeded(repoCtx, reference)
	if err != nil {
		if errors.Is(err, repository.ErrIntentSucceeded) {
			log.InfoContext(ctx, "intent settled by another task, skipping")
			return nil
		}
		return err
	}

	log.InfoContext(ctx, "payment settled and wallet credited",
		slog.Int64("amount_minor", settled.Amount.Minor()),
		slog.String("wallet_id", settled.WalletID),
	)
	p.notify(ctx, repoCtx, log, settled)
	return nil
}

func (p *PaymentWorker) refundLatePayment(ctx context.Context, log *slog.Logger, intent *repository.TransactionIntent) error {
	if p.Provider == nil {
		return fmt.Errorf("%w: no payment provider configured", asynq.SkipRetry)
	}
	repoCtx := p.repoCtx(ctx, log)
	log = log.With(slog.String("intent_status", intent.Status), slog.Int64("amount_minor", intent.Amount.Minor()))

	intentID, err := uuid.Parse(intent.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}


	if intent.Status == intentStatusFailed {
		credited, err := p.Ledger.TransferExists(ctx, intentID)
		if err != nil {
			return err
		}
		if credited {
			settled, err := p.Repository.MarkIntentSucceeded(repoCtx, intent.ID)
			if err != nil {
				if errors.Is(err, repository.ErrIntentSucceeded) {
					return nil
				}
				return err
			}
			log.WarnContext(ctx, "intent was failed after the wallet was credited, marked succeeded")
			p.notify(ctx, repoCtx, log, settled)
			return nil
		}
	}

	if _, err := p.Repository.MarkIntentRefunding(repoCtx, intent.ID); err != nil {
		if errors.Is(err, repository.ErrIntentNotFailed) {
			log.InfoContext(ctx, "intent is no longer awaiting a refund, skipping")
			return nil
		}
		return err
	}

	log.WarnContext(ctx, "payment arrived after the intent failed, refunding")
	if err := p.Provider.Refund(ctx, intent.ID, intent.Amount); err != nil {
		return err
	}

	if _, err := p.Repository.MarkIntentRefunded(repoCtx, intent.ID); err != nil {
		if errors.Is(err, repository.ErrIntentNotFailed) {
			return nil
		}
		return err
	}
	log.InfoContext(ctx, "late payment refunded")
	return nil
}

func (p *PaymentWorker) fail(ctx context.Context, log *slog.Logger, reference string) error {
	if _, err := p.Repository.MarkIntentFailed(p.repoCtx(ctx, log), reference); err != nil {
		if errors.Is(err, repository.ErrIntentNotPending) {
			log.InfoContext(ctx, "intent already resolved, nothing to fail")
			return nil
		}
		return err
	}
	log.InfoContext(ctx, "payment marked failed")
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
