package services

import (
	"net/http"
	"sort"
	"time"

	"github/marveldo/eda-monolith/internal/ledger"
	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/shared"

	"github.com/google/uuid"
)


const MaxHistoryScan = 500

const ProviderInternal = "wallet"

func (s *Service) ListMyTransactions(ctx *ServiceCtx, page *ActivityPageParam) (*TransactionList, *shared.AppError) {
	ctx, span := ctx.Start("payment.transaction.list")
	defer span.End()

	userID, appErr := CallerID(ctx)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	limit, offset := page.Normalise()
	repoCtx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	scan := offset + limit
	if scan > MaxHistoryScan {
		scan = MaxHistoryScan
	}

	settled, appErr := s.settledTransactions(ctx, repoCtx, userID, scan)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	unsettled, appErr := s.unsettledTransactions(ctx, repoCtx, userID, scan)
	if appErr != nil {
		return nil, ctx.Fail(appErr)
	}

	merged := append(settled, unsettled...)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].sortKey.After(merged[j].sortKey)
	})

	transactions := make([]Transaction, 0, limit)
	for i := offset; i < len(merged) && len(transactions) < limit; i++ {
		transactions = append(transactions, merged[i].Transaction)
	}

	return &TransactionList{
		Transactions: transactions,
		// Total counts the merged window, not every row that has ever
		// existed: an exact count would mean scanning both sources in full.
		Total:  int64(len(merged)),
		Limit:  limit,
		Offset: offset,
	}, nil
}

type sortedTransaction struct {
	Transaction
	sortKey time.Time
}

func (s *Service) settledTransactions(ctx *ServiceCtx, repoCtx *repository.RepoCtx, userID string, scan int) ([]sortedTransaction, *shared.AppError) {
	if s.Ledger == nil {
		return nil, nil
	}

	wallets, err := s.Repository.ListWalletsByUser(repoCtx, userID)
	if err != nil {
		return nil, &shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusInternalServerError}
	}
	if len(wallets) == 0 {
		return nil, nil
	}

	walletIDs := make([]uuid.UUID, 0, len(wallets))
	walletByID := make(map[uuid.UUID]*repository.Wallet, len(wallets))
	for _, wallet := range wallets {
		parsed, err := uuid.Parse(wallet.ID)
		if err != nil {
			return nil, &shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusInternalServerError}
		}
		walletIDs = append(walletIDs, parsed)
		walletByID[parsed] = wallet
	}

	movements, err := s.Ledger.WalletMovements(ctx.Context, walletIDs, uint32(scan))
	if err != nil {
		return nil, &shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusBadGateway}
	}
	depositIDs := make([]string, 0, len(movements))
	for _, movement := range movements {
		if movement.Code == ledger.TransferDeposit {
			depositIDs = append(depositIDs, movement.TransferID.String())
		}
	}
	intents, err := s.Repository.GetIntentsByIDs(repoCtx, depositIDs)
	if err != nil {
		return nil, &shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusInternalServerError}
	}

	result := make([]sortedTransaction, 0, len(movements))
	for _, movement := range movements {
		reference := movement.TransferID.String()

		kind := TransactionKindTransfer
		provider := ProviderInternal
		if movement.Code == ledger.TransferDeposit {
			kind = TransactionKindDeposit
			if intent, ok := intents[reference]; ok {
				provider = intent.Provider
			} else {
				provider = ""
			}
		}

		at := movement.Time()
		result = append(result, sortedTransaction{
			Transaction: Transaction{
				Reference: reference,
				UserID:    userID,
				WalletID:  movement.WalletID.String(),
				Amount:    movement.Amount,
				Status:    string(TransactionStatusSuccess),
				Provider:  provider,
				Kind:      kind,
				Direction: string(movement.Direction),
				CreatedAt: at.Format(TimeLayout),
			},
			sortKey: at,
		})
	}
	return result, nil
}


func (s *Service) unsettledTransactions(ctx *ServiceCtx, repoCtx *repository.RepoCtx, userID string, scan int) ([]sortedTransaction, *shared.AppError) {
	intents, err := s.Repository.ListUnsettledIntents(repoCtx, userID, scan)
	if err != nil {
		return nil, &shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusInternalServerError}
	}
	transfers, err := s.Repository.ListUnsettledTransfers(repoCtx, userID, scan)
	if err != nil {
		return nil, &shared.AppError{Message: "Could Not Load Transactions", Err: err, Code: http.StatusInternalServerError}
	}

	result := make([]sortedTransaction, 0, len(intents)+len(transfers))
	for _, intent := range intents {
		result = append(result, sortedTransaction{
			Transaction: Transaction{
				Reference: intent.ID,
				UserID:    intent.UserID,
				WalletID:  intent.WalletID,
				Amount:    intent.Amount,
				Status:    intent.Status,
				Provider:  intent.Provider,
				Kind:      TransactionKindDeposit,
				Direction: string(ledger.DirectionCredit),
				CreatedAt: intent.CreatedAt.UTC().Format(TimeLayout),
			},
			sortKey: intent.CreatedAt,
		})
	}

	for _, transfer := range transfers {
		direction := ledger.DirectionDebit
		walletID := transfer.SenderWalletID
		if transfer.RecipientUserID == userID {
			direction = ledger.DirectionCredit
			walletID = transfer.RecipientWalletID
		}

		result = append(result, sortedTransaction{
			Transaction: Transaction{
				Reference: transfer.ID,
				UserID:    userID,
				WalletID:  walletID,
				Amount:    transfer.Amount,
				Status:    transfer.Status,
				Provider:  ProviderInternal,
				Kind:      TransactionKindTransfer,
				Direction: string(direction),
				CreatedAt: transfer.CreatedAt.UTC().Format(TimeLayout),
			},
			sortKey: transfer.CreatedAt,
		})
	}
	return result, nil
}
