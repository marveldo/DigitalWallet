package services

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github/marveldo/eda-monolith/internal/ledger"
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/shared"

	"github.com/google/uuid"
)

// CreateWallet opens a wallet in the given currency for the caller and creates
// its ledger account. A user holds at most one wallet per currency.
func (s *Service) CreateWallet(ctx *ServiceCtx, currency string) (*Wallet, *shared.AppError) {
	ctx, span := ctx.Start("wallet.create")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = DefaultDepositCurrency
	}
	if _, err := ledger.LedgerForCurrency(currency); err != nil {
		return nil, ctx.Fail(&shared.AppError{Message: "Unsupported Currency", Err: err, Code: http.StatusBadRequest})
	}

	log := ctx.Logger.With(slog.String("service", "wallet.create"), slog.String("user_id", userID), slog.String("currency", currency))
	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	_, err := s.Repository.GetWalletByUser(repoCtx, userID, currency)
	if err == nil {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("wallet already exists for currency"),
			Message: "You Already Have A " + currency + " Wallet",
			Code:    http.StatusConflict,
		})
	}
	if !errors.Is(err, repository.ErrNotFound) {
		log.Error("could not check for an existing wallet", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Create Wallet", Err: err, Code: http.StatusInternalServerError})
	}

	wallet, err := s.Repository.CreateWallet(repoCtx, userID, currency)
	if err != nil {
		log.Error("could not persist wallet", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Create Wallet", Err: err, Code: http.StatusInternalServerError})
	}
	log.Info("wallet created", slog.String("wallet_id", wallet.ID))

	s.ensureLedgerAccount(ctx, wallet)
	return &Wallet{ID: wallet.ID, Currency: wallet.Currency, Status: wallet.Status}, nil
}

// ListMyWallets returns the caller's wallets with their ledger balances.
func (s *Service) ListMyWallets(ctx *ServiceCtx) ([]Wallet, *shared.AppError) {
	ctx, span := ctx.Start("wallet.list")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})
	wallets, err := s.Repository.ListWalletsByUser(repoCtx, userID)
	if err != nil {
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallets", Err: err, Code: http.StatusInternalServerError})
	}
	return s.withBalances(ctx, wallets)
}

// GetWallet returns one of the caller's wallets with its ledger balance. A
// wallet owned by someone else is reported as not found.
func (s *Service) GetWallet(ctx *ServiceCtx, walletID string) (*Wallet, *shared.AppError) {
	ctx, span := ctx.Start("wallet.get")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})
	wallet, err := s.Repository.GetWalletByID(repoCtx, walletID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ctx.Fail(&shared.AppError{Message: "Wallet Not Found", Err: err, Code: http.StatusNotFound})
		}
		return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallet", Err: err, Code: http.StatusInternalServerError})
	}
	if wallet.UserID != userID {
		return nil, ctx.Fail(&shared.AppError{
			Err:     errors.New("caller does not own this wallet"),
			Message: "Wallet Not Found",
			Code:    http.StatusNotFound,
		})
	}

	result, appErr := s.withBalances(ctx, []*repository.Wallet{wallet})
	if appErr != nil {
		return nil, appErr
	}
	return &result[0], nil
}

func (s *Service) withBalances(ctx *ServiceCtx, wallets []*repository.Wallet) ([]Wallet, *shared.AppError) {
	ids := make([]uuid.UUID, 0, len(wallets))
	for _, wallet := range wallets {
		id, err := uuid.Parse(wallet.ID)
		if err != nil {
			return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallets", Err: err, Code: http.StatusInternalServerError})
		}
		ids = append(ids, id)
	}

	balances := map[uuid.UUID]shared.Money{}
	if s.Ledger != nil {
		var err error
		balances, err = s.Ledger.Balances(ctx.Context, ids)
		if err != nil {
			return nil, ctx.Fail(&shared.AppError{Message: "Could Not Load Wallet Balance", Err: err, Code: http.StatusBadGateway})
		}
	}

	result := make([]Wallet, 0, len(wallets))
	for i, wallet := range wallets {
		result = append(result, Wallet{
			ID:       wallet.ID,
			Currency: wallet.Currency,
			Status:   wallet.Status,
			Balance:  balances[ids[i]],
		})
	}
	return result, nil
}

// ensureLedgerAccount opens the wallet's ledger account. A failure is logged
// but not returned: the wallet row already exists, and Deposit ensures the
// account again before crediting it.
func (s *Service) ensureLedgerAccount(ctx *ServiceCtx, wallet *repository.Wallet) {
	if s.Ledger == nil {
		return
	}
	walletID, err := uuid.Parse(wallet.ID)
	if err != nil {
		ctx.Logger.Error("wallet has an invalid id", slog.String("wallet_id", wallet.ID), slog.Any("error", err))
		return
	}
	if err := s.Ledger.EnsureWalletAccount(ctx.Context, walletID, wallet.Currency); err != nil {
		ctx.Logger.Warn("could not open ledger account for wallet, deposit will retry",
			slog.String("wallet_id", wallet.ID), slog.Any("error", err))
	}
}
