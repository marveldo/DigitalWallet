package queues

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github/marveldo/eda-monolith/internal/ledger"
	"github/marveldo/eda-monolith/internal/repository"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TaskTypeTransferExecute = "transfer:execute"

	TransferMaxRetry = 10
)

type TransferPayload struct {
	TransferID string `json:"transfer_id"`
}

type TransferWorker struct {
	*asynq.Client
	*repository.Repository
	Logger   *slog.Logger
	Queue    string
	Ledger   *ledger.Ledger
	Activity *ActivityWorker
}

type TransferWorkerConfig struct {
	Client     *asynq.Client
	Repository *repository.Repository
	Logger     *slog.Logger
	Queue      string
	Ledger     *ledger.Ledger
	Activity   *ActivityWorker
}

func NewTransferWorker(cfg *TransferWorkerConfig) *TransferWorker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &TransferWorker{
		Client:     cfg.Client,
		Repository: cfg.Repository,
		Logger:     logger.With(slog.String("worker", "transfer")),
		Queue:      cfg.Queue,
		Ledger:     cfg.Ledger,
		Activity:   cfg.Activity,
	}
}

func (t *TransferWorker) EnqueueWithContext(task *asynq.Task, ctx context.Context) (*asynq.TaskInfo, error) {
	return EnqueueWithContext(task, ctx, t.Queue, t.Client, t.Logger)
}

func ExecuteTaskID(transferID string) string {
	return TaskTypeTransferExecute + ":" + transferID
}

func (t *TransferWorker) GenerateNewTask(name string, payload TransferPayload) (*asynq.Task, error) {
	switch name {
	case TaskTypeTransferExecute:
		return GenerateNewTask(name, payload,
			asynq.MaxRetry(TransferMaxRetry),
			asynq.TaskID(ExecuteTaskID(payload.TransferID)),
		)
	default:
		return GenerateNewTask(name, payload)
	}
}

func (t *TransferWorker) decode(task *asynq.Task) (TransferPayload, *slog.Logger, error) {
	var payload TransferPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return payload, t.Logger, fmt.Errorf("%w: unprocessable transfer payload: %v", asynq.SkipRetry, err)
	}
	log := t.Logger.With(
		slog.String("task_type", task.Type()),
		slog.String("transfer_id", payload.TransferID),
	)
	return payload, log, nil
}

func (t *TransferWorker) repoCtx(ctx context.Context, log *slog.Logger) *repository.RepoCtx {
	return t.Repository.NewRepoCtx(repository.RepoctxConfig{Context: ctx, Logger: log})
}


func (t *TransferWorker) HandleTransferExecute(ctx context.Context, task *asynq.Task) error {
	payload, log, err := t.decode(task)
	if err != nil {
		return err
	}
	if t.Ledger == nil {
		return fmt.Errorf("%w: no ledger configured", asynq.SkipRetry)
	}
	repoCtx := t.repoCtx(ctx, log)

	transfer, err := t.Repository.GetTransferByID(repoCtx, payload.TransferID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("%w: no transfer with this id: %v", asynq.SkipRetry, err)
		}
		return err
	}
	if transfer.Status != intentStatusPending {
		log.InfoContext(ctx, "transfer already resolved, skipping", slog.String("status", transfer.Status))
		return nil
	}

	sender, recipient, err := t.loadWallets(ctx, repoCtx, log, transfer)
	if err != nil {
		return err
	}
	if sender == nil {
		return nil
	}

	transferID, err := uuid.Parse(transfer.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	senderWalletID, err := uuid.Parse(sender.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	recipientWalletID, err := uuid.Parse(recipient.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	err = t.Ledger.Transfer(ctx, ledger.TransferParam{
		TransferID:          transferID,
		SenderWalletID:      senderWalletID,
		ReceipientWalletID:  recipientWalletID,
		SenderCurrency:      sender.Currency,
		ReceipientsCurrency: recipient.Currency,
		Amount:              transfer.Amount,
	})
	switch {
	case err == nil:
	case errors.Is(err, ledger.ErrInsufficientFunds):

		return t.finalizeFailed(ctx, repoCtx, log, transfer, "Insufficient funds")
	case errors.Is(err, ledger.ErrCrossCurrency), errors.Is(err, ledger.ErrRejected):
		
		if failErr := t.finalizeFailed(ctx, repoCtx, log, transfer, "Rejected by the ledger"); failErr != nil {
			return failErr
		}
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	default:
		// Transient. The row stays pending and the task is retried.
		return err
	}

	settled, err := t.Repository.MarkTransferSucceeded(repoCtx, transfer.ID)
	if err != nil {
		if errors.Is(err, repository.ErrTransferNotPending) {
			log.InfoContext(ctx, "transfer closed by another task, skipping")
			return nil
		}
		return err
	}

	log.InfoContext(ctx, "transfer completed",
		slog.Int64("amount_minor", settled.Amount.Minor()),
		slog.String("sender_wallet_id", settled.SenderWalletID),
		slog.String("recipient_wallet_id", settled.RecipientWalletID),
	)
	t.notify(ctx, log, settled)
	return nil
}

func (t *TransferWorker) loadWallets(ctx context.Context, repoCtx *repository.RepoCtx, log *slog.Logger, transfer *repository.Transfer) (*repository.Wallet, *repository.Wallet, error) {
	sender, err := t.Repository.GetWalletByID(repoCtx, transfer.SenderWalletID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, fmt.Errorf("%w: sending wallet is gone: %v", asynq.SkipRetry, err)
		}
		return nil, nil, err
	}
	recipient, err := t.Repository.GetWalletByID(repoCtx, transfer.RecipientWalletID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, fmt.Errorf("%w: receiving wallet is gone: %v", asynq.SkipRetry, err)
		}
		return nil, nil, err
	}

	if sender.Status != walletStatusActive || recipient.Status != walletStatusActive {
		log.WarnContext(ctx, "a wallet stopped being active before the transfer ran",
			slog.String("sender_status", sender.Status),
			slog.String("recipient_status", recipient.Status),
		)
		return nil, nil, t.finalizeFailed(ctx, repoCtx, log, transfer, "An account involved is not active")
	}
	if !strings.EqualFold(sender.Currency, recipient.Currency) {
		return nil, nil, t.finalizeFailed(ctx, repoCtx, log, transfer, "Cross-currency transfers are not supported")
	}
	return sender, recipient, nil
}

func (t *TransferWorker) finalizeFailed(ctx context.Context, repoCtx *repository.RepoCtx, log *slog.Logger, transfer *repository.Transfer, reason string) error {
	failed, err := t.Repository.MarkTransferFailed(repoCtx, transfer.ID, reason)
	if err != nil {
		if errors.Is(err, repository.ErrTransferNotPending) {
			return nil
		}
		return err
	}
	log.InfoContext(ctx, "transfer failed", slog.String("reason", reason))
	t.record(ctx, log, failed.SenderUserID, fmt.Sprintf("Transfer of %s failed: %s", failed.Amount.String(), reason))
	return nil
}

func (t *TransferWorker) HandleTransferExhausted(ctx context.Context, task *asynq.Task, cause error) {
	payload, log, err := t.decode(task)
	if err != nil {
		log.ErrorContext(ctx, "could not read exhausted transfer task", slog.Any("error", err))
		return
	}
	log = log.With(slog.Any("cause", cause))
	repoCtx := t.repoCtx(ctx, log)

	transfer, err := t.Repository.GetTransferByID(repoCtx, payload.TransferID)
	if err != nil {
		log.ErrorContext(ctx, "could not load the exhausted transfer", slog.Any("error", err))
		return
	}
	if transfer.Status != intentStatusPending {
		return
	}

	transferID, err := uuid.Parse(transfer.ID)
	if err != nil {
		log.ErrorContext(ctx, "exhausted transfer has an invalid id", slog.Any("error", err))
		return
	}

	if t.Ledger != nil {
		moved, err := t.Ledger.TransferExists(ctx, transferID)
		if err != nil {
			
			log.ErrorContext(ctx, "could not check the ledger for an exhausted transfer, leaving it pending",
				slog.Any("error", err))
			return
		}
		if moved {
			settled, err := t.Repository.MarkTransferSucceeded(repoCtx, transfer.ID)
			if err != nil && !errors.Is(err, repository.ErrTransferNotPending) {
				log.ErrorContext(ctx, "could not close out a transfer the ledger had already made", slog.Any("error", err))
				return
			}
			log.WarnContext(ctx, "transfer ran out of retries after the ledger moved the money, marked succeeded")
			if settled != nil {
				t.notify(ctx, log, settled)
			}
			return
		}
	}

	if err := t.finalizeFailed(ctx, repoCtx, log, transfer, "The transfer could not be completed"); err != nil {
		log.ErrorContext(ctx, "could not mark an unresolved transfer failed", slog.Any("error", err))
		return
	}
	log.WarnContext(ctx, "transfer never reached the ledger, marked failed")
}

func (t *TransferWorker) notify(ctx context.Context, log *slog.Logger, transfer *repository.Transfer) {
	t.record(ctx, log, transfer.SenderUserID, fmt.Sprintf("Sent %s", transfer.Amount.String()))
	t.record(ctx, log, transfer.RecipientUserID, fmt.Sprintf("Received %s", transfer.Amount.String()))
}

func (t *TransferWorker) record(ctx context.Context, log *slog.Logger, userID string, action string) {
	if t.Activity == nil {
		return
	}
	task, err := t.Activity.GenerateNewTask(UpdateUserActivity, ActivityPayload{UserID: userID, Action: action})
	if err != nil {
		log.ErrorContext(ctx, "could not build transfer activity task", slog.Any("error", err))
		return
	}
	if _, err := t.Activity.EnqueueWithContext(task, ctx); err != nil {
		log.ErrorContext(ctx, "transfer activity was not queued", slog.Any("error", err))
	}
}
