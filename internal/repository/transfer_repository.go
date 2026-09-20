package repository

import (
	"errors"
	"log/slog"

	"github/marveldo/eda-monolith/internal/repository/db"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

var ErrTransferNotPending = errors.New("transfer is no longer pending")

func (t *TransactionRepository) MapTransferModelToTransfer(model *db.Transfer) *Transfer {
	return &Transfer{
		ID:                model.ID.String(),
		SenderUserID:      model.SenderUserID.String(),
		SenderWalletID:    model.SenderWalletID.String(),
		RecipientUserID:   model.RecipientUserID.String(),
		RecipientWalletID: model.RecipientWalletID.String(),
		Amount:            model.Amount,
		Currency:          string(model.Currency),
		Status:            string(model.Status),
		FailureReason:     model.FailureReason,
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

// GetWalletByAccountNumber resolves the number a user actually types into the
// wallet behind it.
func (t *TransactionRepository) GetWalletByAccountNumber(ctx *RepoCtx, accountNumber string) (*Wallet, error) {
	ctx, span := ctx.Start("transaction.wallet_by_account_number")
	defer span.End()

	var wallet db.Wallet
	err := ctx.DB.WithContext(ctx.Context).Where("account_number = ?", accountNumber).First(&wallet).Error
	if err != nil {
		return nil, ctx.LogError("transaction.wallet_by_account_number", err, slog.String("account_number", accountNumber))
	}
	return t.MapWalletModelToWallet(&wallet), nil
}

func (t *TransactionRepository) CreateTransfer(ctx *RepoCtx, param *CreateTransferParam) (*Transfer, error) {
	ctx, span := ctx.Start("transaction.create_transfer")
	defer span.End()

	senderUser, err := ParseUUID(param.SenderUserID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_transfer", err, slog.String("user_id", param.SenderUserID))
	}
	senderWallet, err := ParseUUID(param.SenderWalletID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_transfer", err, slog.String("wallet_id", param.SenderWalletID))
	}
	recipientUser, err := ParseUUID(param.RecipientUserID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_transfer", err, slog.String("user_id", param.RecipientUserID))
	}
	recipientWallet, err := ParseUUID(param.RecipientWalletID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_transfer", err, slog.String("wallet_id", param.RecipientWalletID))
	}

	transfer := db.Transfer{
		SenderUserID:      senderUser,
		SenderWalletID:    senderWallet,
		RecipientUserID:   recipientUser,
		RecipientWalletID: recipientWallet,
		Amount:            param.Amount,
		Currency:          db.Currency(param.Currency),
		Status:            db.TransactionPending,
	}
	if err := ctx.DB.WithContext(ctx.Context).Create(&transfer).Error; err != nil {
		return nil, ctx.LogError("transaction.create_transfer", err, slog.String("user_id", param.SenderUserID))
	}
	return t.MapTransferModelToTransfer(&transfer), nil
}

func (t *TransactionRepository) GetTransferByID(ctx *RepoCtx, transferID string) (*Transfer, error) {
	ctx, span := ctx.Start("transaction.get_transfer")
	defer span.End()

	parsed, err := ParseUUID(transferID)
	if err != nil {
		return nil, ctx.LogError("transaction.get_transfer", err, slog.String("transfer_id", transferID))
	}

	var transfer db.Transfer
	if err := ctx.DB.WithContext(ctx.Context).Where("id = ?", parsed).First(&transfer).Error; err != nil {
		return nil, ctx.LogError("transaction.get_transfer", err, slog.String("transfer_id", transferID))
	}
	return t.MapTransferModelToTransfer(&transfer), nil
}

// MarkTransferSucceeded closes out a transfer the ledger accepted. The status
// guard makes it safe to run twice: a redelivered task finds no pending row
// and gets ErrTransferNotPending rather than re-notifying both parties.
func (t *TransactionRepository) MarkTransferSucceeded(ctx *RepoCtx, transferID string) (*Transfer, error) {
	return t.markTransfer(ctx, "transaction.mark_transfer_succeeded", transferID, db.TransactionSuccess, "")
}

func (t *TransactionRepository) MarkTransferFailed(ctx *RepoCtx, transferID string, reason string) (*Transfer, error) {
	return t.markTransfer(ctx, "transaction.mark_transfer_failed", transferID, db.TransactionFailed, reason)
}

func (t *TransactionRepository) markTransfer(ctx *RepoCtx, op string, transferID string, status db.TransactionStatus, reason string) (*Transfer, error) {
	ctx, span := ctx.Start(op)
	defer span.End()

	parsed, err := ParseUUID(transferID)
	if err != nil {
		return nil, ctx.LogError(op, err, slog.String("transfer_id", transferID))
	}

	var transfer db.Transfer
	result := ctx.DB.WithContext(ctx.Context).
		Model(&transfer).
		Clauses(clause.Returning{}).
		Where("id = ? AND status = ?", parsed, db.TransactionPending).
		Updates(map[string]any{"status": status, "failure_reason": reason})
	if result.Error != nil {
		return nil, ctx.LogError(op, result.Error, slog.String("transfer_id", transferID))
	}
	if result.RowsAffected == 0 {
		return nil, ErrTransferNotPending
	}
	return t.MapTransferModelToTransfer(&transfer), nil
}

// ListTransfersByUser returns the transfers a user sent or received, newest first.
func (t *TransactionRepository) ListTransfersByUser(ctx *RepoCtx, userID string, limit int, offset int) ([]*Transfer, int64, error) {
	ctx, span := ctx.Start("transaction.list_transfers")
	defer span.End()

	parsed, err := ParseUUID(userID)
	if err != nil {
		return nil, 0, ctx.LogError("transaction.list_transfers", err, slog.String("user_id", userID))
	}

	query := ctx.DB.WithContext(ctx.Context).
		Model(&db.Transfer{}).
		Where("sender_user_id = ? OR recipient_user_id = ?", parsed, parsed)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, ctx.LogError("transaction.list_transfers", err, slog.String("user_id", userID))
	}

	var models []db.Transfer
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, ctx.LogError("transaction.list_transfers", err, slog.String("user_id", userID))
	}

	transfers := make([]*Transfer, 0, len(models))
	for i := range models {
		transfers = append(transfers, t.MapTransferModelToTransfer(&models[i]))
	}
	return transfers, total, nil
}

// ListUnsettledIntents returns the deposits that never reached the ledger:
// still pending, failed, or refunded. The ledger has no record of these, so
// they are the half of a user's history TigerBeetle cannot supply.
func (t *TransactionRepository) ListUnsettledIntents(ctx *RepoCtx, userID string, limit int) ([]*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.list_unsettled_intents")
	defer span.End()

	parsed, err := ParseUUID(userID)
	if err != nil {
		return nil, ctx.LogError("transaction.list_unsettled_intents", err, slog.String("user_id", userID))
	}

	var models []db.TransactionIntent
	err = ctx.DB.WithContext(ctx.Context).
		Where("user_id = ? AND status <> ?", parsed, db.TransactionSuccess).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, ctx.LogError("transaction.list_unsettled_intents", err, slog.String("user_id", userID))
	}

	intents := make([]*TransactionIntent, 0, len(models))
	for i := range models {
		intents = append(intents, t.MapIntentModelToIntent(&models[i]))
	}
	return intents, nil
}

func (t *TransactionRepository) ListUnsettledTransfers(ctx *RepoCtx, userID string, limit int) ([]*Transfer, error) {
	ctx, span := ctx.Start("transaction.list_unsettled_transfers")
	defer span.End()

	parsed, err := ParseUUID(userID)
	if err != nil {
		return nil, ctx.LogError("transaction.list_unsettled_transfers", err, slog.String("user_id", userID))
	}

	var models []db.Transfer
	err = ctx.DB.WithContext(ctx.Context).
		Where("(sender_user_id = ? OR recipient_user_id = ?) AND status <> ?", parsed, parsed, db.TransactionSuccess).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, ctx.LogError("transaction.list_unsettled_transfers", err, slog.String("user_id", userID))
	}

	transfers := make([]*Transfer, 0, len(models))
	for i := range models {
		transfers = append(transfers, t.MapTransferModelToTransfer(&models[i]))
	}
	return transfers, nil
}

// GetIntentsByIDs loads the deposits behind a batch of ledger movements, so
// the provider can be filled in without a query per row.
func (t *TransactionRepository) GetIntentsByIDs(ctx *RepoCtx, ids []string) (map[string]*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.intents_by_ids")
	defer span.End()

	found := make(map[string]*TransactionIntent, len(ids))
	if len(ids) == 0 {
		return found, nil
	}

	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if value, err := ParseUUID(id); err == nil {
			parsed = append(parsed, value)
		}
	}
	if len(parsed) == 0 {
		return found, nil
	}

	var models []db.TransactionIntent
	if err := ctx.DB.WithContext(ctx.Context).Where("id IN ?", parsed).Find(&models).Error; err != nil {
		return nil, ctx.LogError("transaction.intents_by_ids", err)
	}
	for i := range models {
		intent := t.MapIntentModelToIntent(&models[i])
		found[intent.ID] = intent
	}
	return found, nil
}
