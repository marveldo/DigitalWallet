package services

import (
	"errors"
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/internal/api/events"
	"github/marveldo/eda-monolith/internal/ledger"
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/internal/repository/db"
	"github/marveldo/eda-monolith/shared"
)

const (
	DefaultDepositCurrency = "NGN"
	MinDepositMinor        = 100
	MaxDepositMinor        = 1_000_000_00
)

func (s *Service) MapIntentToServiceDomain(intent *repository.TransactionIntent) Transaction {
	return Transaction{
		Reference: intent.ID,
		UserID:    intent.UserID,
		WalletID:  intent.WalletID,
		Amount:    intent.Amount,
		Status:    intent.Status,
		Provider:  intent.Provider,
		Kind:      TransactionKindDeposit,
		Direction: string(ledger.DirectionCredit),
		CreatedAt: intent.CreatedAt.UTC().Format(TimeLayout),
	}
}

func (s *Service) InitializeDeposit(ctx *ServiceCtx, param *InitializeDepositParam) (*DepositInitialized, *shared.AppError) {
	ctx, span := ctx.Start("payment.deposit.initialize")
	defer span.End()

	if s.PaymentProvider == nil {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("no payment provider configured"),
			Message: "Payments Are Unavailable",
			Code:    http.StatusServiceUnavailable,
		})
	}

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	amount := param.Money()
	if amount.Minor() < MinDepositMinor || amount.Minor() > MaxDepositMinor {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("deposit amount out of range"),
			Message: "Deposit Amount Is Out Of Range",
			Code:    http.StatusUnprocessableEntity,
		})
	}

	log := ctx.Logger.With(
		slog.String("service", "payment.deposit.initialize"),
		slog.String("user_id", userID),
		slog.Int64("amount_minor", amount.Minor()),
	)

	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	user, err := s.Repository.GetUserByID(repoCtx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "User Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load User", Err: err, Code: http.StatusInternalServerError})
	}

	wallet, err := s.Repository.GetWalletByID(repoCtx, param.WalletID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "Wallet Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallet", Err: err, Code: http.StatusInternalServerError})
	}
	if wallet.UserID != userID {
		log.Warn("deposit rejected, caller does not own the wallet", slog.String("wallet_id", param.WalletID))
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("caller does not own this wallet"),
			Message: "Wallet Not Found",
			Code:    http.StatusNotFound,
		})
	}
	if wallet.Status != string(db.WalletActive) {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("wallet is not active"),
			Message: "Wallet Is Not Active",
			Code:    http.StatusUnprocessableEntity,
		})
	}

	intent, err := s.Repository.CreateIntent(repoCtx, &repository.CreateIntentParam{
		UserID:   userID,
		WalletID: wallet.ID,
		Amount:   amount,
		Provider: s.PaymentProvider.Name(),
	})
	if err != nil {
		log.Error("could not record transaction intent", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Start Deposit", Err: err, Code: http.StatusInternalServerError})
	}

	log = log.With(slog.String("reference", intent.ID))
	initialized, err := s.PaymentProvider.Initialize(ctx.Context, shared.InitializePaymentRequest{
		Reference:   intent.ID,
		Amount:      amount,
		Currency:    wallet.Currency,
		Email:       user.Email,
		CallbackURL: s.PaymentCallbackURL,
		Metadata: map[string]string{
			"user_id":   userID,
			"wallet_id": wallet.ID,
		},
	})
	if err != nil {
		log.Error("payment provider would not start the checkout", slog.Any("error", err),slog.Any("currency", wallet.Currency))
		if _, failErr := s.Repository.MarkIntentFailed(repoCtx, intent.ID); failErr != nil {
			log.Error("could not mark the intent failed", slog.Any("error", failErr))
		}
		code := http.StatusBadGateway
		if errors.Is(err, shared.ErrProviderUnavailable) {
			code = http.StatusServiceUnavailable
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Start Deposit", Err: err, Code: code})
	}

	s.PublishEvent(ctx, events.EventPaymentInitialized, events.PaymentInitializedPayload{
		UserID:      userID,
		Reference:   intent.ID,
		Provider:    s.PaymentProvider.Name(),
		AmountMinor: amount.Minor(),
		Currency:    wallet.Currency,
	})

	log.Info("deposit initialized")
	return &DepositInitialized{
		Reference:        intent.ID,
		AuthorizationURL: initialized.AuthorizationURL,
		AccessCode:       initialized.AccessCode,
		Amount:           amount,
		Currency:         wallet.Currency,
		Status:           intent.Status,
	}, nil
}

func (s *Service) HandlePaymentWebhook(ctx *ServiceCtx, signature string, body []byte) *shared.AppError {
	ctx, span := ctx.Start("payment.webhook")
	defer span.End()

	if s.PaymentProvider == nil {
		return ctx.Fail(&shared.AppError{
			Err:     errors.New("no payment provider configured"),
			Message: "Payments Are Unavailable",
			Code:    http.StatusServiceUnavailable,
		})
	}

	log := ctx.Logger.With(slog.String("service", "payment.webhook"))

	if err := s.PaymentProvider.VerifyWebhookSignature(signature, body); err != nil {
		log.Warn("rejected an unsigned or forged webhook", slog.Any("error", err))
		return ctx.Fail(&shared.AppError{Message: "Invalid Signature", Err: err, Code: http.StatusUnauthorized})
	}

	event, err := s.PaymentProvider.ParseWebhook(body)
	if err != nil {
		if errors.Is(err, shared.ErrUnhandledWebhook) {
			log.Info("ignoring webhook we do not act on", slog.Any("reason", err))
			return nil
		}
		log.Error("could not parse webhook", slog.Any("error", err))
		return ctx.Fail(&shared.AppError{Message: "Unprocessable Webhook", Err: err, Code: http.StatusBadRequest})
	}

	s.PublishEvent(ctx, events.EventPaymentWebhookReceived, events.PaymentWebhookPayload{
		Reference:   event.Reference,
		Provider:    s.PaymentProvider.Name(),
		Event:       event.Event,
		Status:      string(event.Status),
		AmountMinor: event.Amount.Minor(),
		Currency:    event.Currency,
	})

	log.Info("payment webhook accepted",
		slog.String("reference", event.Reference),
		slog.String("event", event.Event),
	)
	return nil
}

func (s *Service) GetTransaction(ctx *ServiceCtx, reference string) (*Transaction, *shared.AppError) {
	ctx, span := ctx.Start("payment.transaction.get")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})
	intent, err := s.Repository.GetIntentByReference(repoCtx, reference)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// The list returns transfer references alongside deposit ones, so
			// a reference that is not a deposit is looked for as a transfer
			// rather than 404ing on a row the client was just shown.
			return s.transferAsTransaction(ctx, repoCtx, userID, reference)
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Transaction", Err: err, Code: http.StatusInternalServerError})
	}

	if intent.UserID != userID {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("caller does not own this transaction"),
			Message: "Transaction Not Found",
			Code:    http.StatusNotFound,
		})
	}

	transaction := s.MapIntentToServiceDomain(intent)
	return &transaction, nil
}

// transferAsTransaction renders a transfer in the transaction shape, so one
// endpoint answers for both kinds of reference.
func (s *Service) transferAsTransaction(ctx *ServiceCtx, repoCtx *repository.RepoCtx, userID string, reference string) (*Transaction, *shared.AppError) {
	transfer, err := s.Repository.GetTransferByID(repoCtx, reference)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "Transaction Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Transaction", Err: err, Code: http.StatusInternalServerError})
	}
	if transfer.SenderUserID != userID && transfer.RecipientUserID != userID {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("caller is not party to this transfer"),
			Message: "Transaction Not Found",
			Code:    http.StatusNotFound,
		})
	}

	direction := ledger.DirectionDebit
	walletID := transfer.SenderWalletID
	if transfer.RecipientUserID == userID {
		direction = ledger.DirectionCredit
		walletID = transfer.RecipientWalletID
	}

	return &Transaction{
		Reference: transfer.ID,
		UserID:    userID,
		WalletID:  walletID,
		Amount:    transfer.Amount,
		Status:    transfer.Status,
		Provider:  ProviderInternal,
		Kind:      TransactionKindTransfer,
		Direction: string(direction),
		CreatedAt: transfer.CreatedAt.UTC().Format(TimeLayout),
	}, nil
}
