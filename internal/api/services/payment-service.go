package services

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github/marveldo/eda-monolith/internal/api/events"
	"github/marveldo/eda-monolith/internal/repository"
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

	currency := strings.ToUpper(strings.TrimSpace(param.Currency))
	if currency == "" {
		currency = DefaultDepositCurrency
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

	wallet, err := s.Repository.GetWalletByUser(repoCtx, userID, currency)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{
				Message: "No " + currency + " Wallet Found For This Account",
				Err:     err,
				Code:    http.StatusNotFound,
			})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallet", Err: err, Code: http.StatusInternalServerError})
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
		log.Error("payment provider would not start the checkout", slog.Any("error", err))
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
		Reference: event.Reference,
		Provider:  s.PaymentProvider.Name(),
		Event:     event.Event,
		Status:    string(event.Status),
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
			return nil, ctx.Fail(&shared.AppError{Message: "Transaction Not Found", Err: err, Code: http.StatusNotFound})
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

func (s *Service) ListMyTransactions(ctx *ServiceCtx, page *ActivityPageParam) (*TransactionList, *shared.AppError) {
	ctx, span := ctx.Start("payment.transaction.list")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	limit, offset := page.Normalise()
	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	result, err := s.Repository.ListUserIntents(repoCtx, userID, limit, offset)
	if err != nil {
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusInternalServerError})
	}

	transactions := make([]Transaction, 0, len(result.Intents))
	for _, intent := range result.Intents {
		transactions = append(transactions, s.MapIntentToServiceDomain(intent))
	}
	return &TransactionList{
		Transactions: transactions,
		Total:        result.Total,
		Limit:        limit,
		Offset:       offset,
	}, nil
}
