package services

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github/marveldo/eda-monolith/internal/api/events"
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/internal/repository/db"
	"github/marveldo/eda-monolith/shared"

	"github.com/google/uuid"
)

const (
	MinTransferMinor = 100
	MaxTransferMinor = 1_000_000_00
)

func (s *Service) MapTransferToServiceDomain(transfer *repository.Transfer) Transfer {
	return Transfer{
		ID:                transfer.ID,
		SenderWalletID:    transfer.SenderWalletID,
		RecipientWalletID: transfer.RecipientWalletID,
		Amount:            transfer.Amount,
		Currency:          transfer.Currency,
		Status:            transfer.Status,
		FailureReason:     transfer.FailureReason,
		CreatedAt:         transfer.CreatedAt.UTC().Format(TimeLayout),
	}
}

func (s *Service) InitiateTransfer(ctx *ServiceCtx, param *InitiateTransferParam) (*Transfer, *shared.AppError) {
	ctx, span := ctx.Start("transfer.initiate")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	amount := param.Money()
	if amount.Minor() < MinTransferMinor || amount.Minor() > MaxTransferMinor {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("transfer amount out of range"),
			Message: "Transfer Amount Is Out Of Range",
			Code:    http.StatusUnprocessableEntity,
		})
	}

	log := ctx.Logger.With(
		slog.String("service", "transfer.initiate"),
		slog.String("user_id", userID),
		slog.Int64("amount_minor", amount.Minor()),
	)
	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	sender, err := s.Repository.GetWalletByID(repoCtx, param.SenderWalletID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "Wallet Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallet", Err: err, Code: http.StatusInternalServerError})
	}
	if sender.UserID != userID {
		log.Warn("transfer rejected, caller does not own the sending wallet", slog.String("wallet_id", param.SenderWalletID))
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("caller does not own this wallet"),
			Message: "Wallet Not Found",
			Code:    http.StatusNotFound,
		})
	}
	if sender.Status != string(db.WalletActive) {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("sending wallet is not active"),
			Message: "Wallet Is Not Active",
			Code:    http.StatusUnprocessableEntity,
		})
	}

	accountNumber := strings.TrimSpace(param.RecipientAccountNumber)
	recipient, err := s.Repository.GetWalletByAccountNumber(repoCtx, accountNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "Account Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Account", Err: err, Code: http.StatusInternalServerError})
	}
	if recipient.ID == sender.ID {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("sender and recipient are the same wallet"),
			Message: "You Cannot Transfer To The Same Wallet",
			Code:    http.StatusBadRequest,
		})
	}
	if !strings.EqualFold(recipient.Currency, sender.Currency) {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("sender and recipient wallets hold different currencies"),
			Message: "Cross-Currency Transfers Are Not Supported Yet",
			Code:    http.StatusBadRequest,
		})
	}
	if recipient.Status != string(db.WalletActive) {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("recipient wallet is not active"),
			Message: "That Account Cannot Receive Money",
			Code:    http.StatusUnprocessableEntity,
		})
	}

	if appErr := s.checkSufficientBalance(ctx, sender.ID, amount); appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	transfer, err := s.Repository.CreateTransfer(repoCtx, &repository.CreateTransferParam{
		SenderUserID:      userID,
		SenderWalletID:    sender.ID,
		RecipientUserID:   recipient.UserID,
		RecipientWalletID: recipient.ID,
		Amount:            amount,
		Currency:          sender.Currency,
	})
	if err != nil {
		log.Error("could not record transfer", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Start Transfer", Err: err, Code: http.StatusInternalServerError})
	}

	log.Info("transfer accepted", slog.String("transfer_id", transfer.ID))
	s.PublishEvent(ctx, events.EventTransferInitiated, events.TransferInitiatedPayload{
		TransferID:  transfer.ID,
		UserID:      userID,
		AmountMinor: amount.Minor(),
		Currency:    sender.Currency,
	})

	result := s.MapTransferToServiceDomain(transfer)
	return &result, nil
}
func (s *Service) checkSufficientBalance(ctx *ServiceCtx, walletID string, amount shared.Money) *shared.AppError {
	if s.Ledger == nil {
		return nil
	}
	parsed, err := uuid.Parse(walletID)
	if err != nil {
		return &shared.AppError{Message: "Could Not Load Wallet Balance", Err: err, Code: http.StatusInternalServerError}
	}
	balances, err := s.Ledger.Balances(ctx.Context, []uuid.UUID{parsed})
	if err != nil {
		ctx.Logger.Warn("could not pre-check the sender balance", slog.Any("error", err))
		return nil
	}
	if balances[parsed] < amount {
		return &shared.AppError{
			Err:     errors.New("sender balance is below the transfer amount"),
			Message: "Insufficient Funds",
			Code:    http.StatusUnprocessableEntity,
		}
	}
	return nil
}

func (s *Service) GetTransfer(ctx *ServiceCtx, transferID string) (*Transfer, *shared.AppError) {
	ctx, span := ctx.Start("transfer.get")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})
	transfer, err := s.Repository.GetTransferByID(repoCtx, transferID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "Transfer Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Transfer", Err: err, Code: http.StatusInternalServerError})
	}
	if transfer.SenderUserID != userID && transfer.RecipientUserID != userID {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("caller is not party to this transfer"),
			Message: "Transfer Not Found",
			Code:    http.StatusNotFound,
		})
	}

	result := s.MapTransferToServiceDomain(transfer)
	return &result, nil
}
