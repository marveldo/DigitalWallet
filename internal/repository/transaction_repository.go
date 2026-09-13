package repository

import (
	"errors"
	"log/slog"

	"github/marveldo/eda-monolith/internal/repository/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrIntentNotPending = errors.New("transaction intent is no longer pending")
	ErrAmountMismatch   = errors.New("settled amount does not match the intent")
	ErrWalletNotActive  = errors.New("wallet is not active")
)

type TransactionRepository struct{}

type TransactionRepositoryConfig struct{}

func NewTransactionRepository(*TransactionRepositoryConfig) TransactionRepository {
	return TransactionRepository{}
}

func (t *TransactionRepository) MapIntentModelToIntent(model *db.TransactionIntent) *TransactionIntent {
	return &TransactionIntent{
		ID:        model.ID.String(),
		UserID:    model.UserID.String(),
		WalletID:  model.Wallet.String(),
		Amount:    model.Amount,
		Status:    string(model.Status),
		Provider:  model.Provider,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func (t *TransactionRepository) MapWalletModelToWallet(model *db.Wallet) *Wallet {
	return &Wallet{
		ID:       model.ID.String(),
		UserID:   model.UserID.String(),
		Balance:  model.Balance,
		Currency: string(model.Currency),
	}
}

func (t *TransactionRepository) GetWalletByUser(ctx *RepoCtx, userID string, currency string) (*Wallet, error) {
	ctx, span := ctx.Start("transaction.wallet_by_user")
	defer span.End()

	parsedUser, err := ParseUUID(userID)
	if err != nil {
		return nil, ctx.LogError("transaction.wallet_by_user", err, slog.String("user_id", userID))
	}

	query := ctx.DB.WithContext(ctx.Context).Where("user_id = ?", parsedUser)
	if currency != "" {
		query = query.Where("currency = ?", currency)
	}

	var wallet db.Wallet
	if err := query.First(&wallet).Error; err != nil {
		return nil, ctx.LogError("transaction.wallet_by_user", err, slog.String("user_id", userID))
	}
	return t.MapWalletModelToWallet(&wallet), nil
}

func (t *TransactionRepository) CreateIntent(ctx *RepoCtx, param *CreateIntentParam) (*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.create_intent")
	defer span.End()

	parsedUser, err := ParseUUID(param.UserID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_intent", err, slog.String("user_id", param.UserID))
	}
	parsedWallet, err := ParseUUID(param.WalletID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_intent", err, slog.String("wallet_id", param.WalletID))
	}

	intent := db.TransactionIntent{
		UserID:   parsedUser,
		Wallet:   parsedWallet,
		Amount:   param.Amount,
		Provider: param.Provider,
		Status:   db.TransactionPending,
	}
	if err := ctx.DB.WithContext(ctx.Context).Create(&intent).Error; err != nil {
		return nil, ctx.LogError("transaction.create_intent", err)
	}
	return t.MapIntentModelToIntent(&intent), nil
}

func (t *TransactionRepository) GetIntentByReference(ctx *RepoCtx, reference string) (*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.get_intent")
	defer span.End()

	parsed, err := ParseUUID(reference)
	if err != nil {
		return nil, ctx.LogError("transaction.get_intent", err, slog.String("reference", reference))
	}

	var intent db.TransactionIntent
	if err := ctx.DB.WithContext(ctx.Context).Where("id = ?", parsed).First(&intent).Error; err != nil {
		return nil, ctx.LogError("transaction.get_intent", err, slog.String("reference", reference))
	}
	return t.MapIntentModelToIntent(&intent), nil
}

func (t *TransactionRepository) ListUserIntents(ctx *RepoCtx, userID string, limit int, offset int) (*IntentPage, error) {
	ctx, span := ctx.Start("transaction.list_intents")
	defer span.End()

	parsedUser, err := ParseUUID(userID)
	if err != nil {
		return nil, ctx.LogError("transaction.list_intents", err, slog.String("user_id", userID))
	}

	query := ctx.DB.WithContext(ctx.Context).Model(&db.TransactionIntent{}).Where("user_id = ?", parsedUser)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, ctx.LogError("transaction.list_intents.count", err)
	}

	var models []db.TransactionIntent
	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&models).Error
	if err != nil {
		return nil, ctx.LogError("transaction.list_intents", err)
	}

	intents := make([]*TransactionIntent, 0, len(models))
	for i := range models {
		intents = append(intents, t.MapIntentModelToIntent(&models[i]))
	}
	return &IntentPage{Intents: intents, Total: total}, nil
}

func (t *TransactionRepository) MarkIntentFailed(ctx *RepoCtx, reference string) (*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.mark_failed")
	defer span.End()

	parsed, err := ParseUUID(reference)
	if err != nil {
		return nil, ctx.LogError("transaction.mark_failed", err, slog.String("reference", reference))
	}

	var intent db.TransactionIntent
	result := ctx.DB.WithContext(ctx.Context).
		Model(&intent).
		Clauses(clause.Returning{}).
		Where("id = ? AND status = ?", parsed, db.TransactionPending).
		Update("status", db.TransactionFailed)
	if result.Error != nil {
		return nil, ctx.LogError("transaction.mark_failed", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ctx.LogError("transaction.mark_failed", ErrIntentNotPending, slog.String("reference", reference))
	}
	return t.MapIntentModelToIntent(&intent), nil
}

func (t *TransactionRepository) SettleIntent(ctx *RepoCtx, param *SettleIntentParam) (*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.settle")
	defer span.End()

	parsed, err := ParseUUID(param.Reference)
	if err != nil {
		return nil, ctx.LogError("transaction.settle", err, slog.String("reference", param.Reference))
	}

	var settled *TransactionIntent
	err = ctx.DB.WithContext(ctx.Context).Transaction(func(tx *gorm.DB) error {
		var intent db.TransactionIntent
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", parsed).
			First(&intent).Error
		if err != nil {
			return err
		}
		if intent.Status != db.TransactionPending {
			return ErrIntentNotPending
		}
		if param.Amount != 0 && param.Amount != intent.Amount {
			return ErrAmountMismatch
		}

		var wallet db.Wallet
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", intent.Wallet).
			First(&wallet).Error
		if err != nil {
			return err
		}
		if wallet.Status != db.WalletActive {
			return ErrWalletNotActive
		}

		entry := db.LedgerEntries{
			Description: param.Description,
			ReferenceID: param.Reference,
			Type:        db.Deposit,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}

		line := db.LedgerLines{
			EntryID:   entry.ID,
			WalletID:  wallet.ID,
			Amount:    intent.Amount,
			Direction: db.Credit,
		}
		if err := tx.Create(&line).Error; err != nil {
			return err
		}

		err = tx.Model(&db.Wallet{}).
			Where("id = ?", wallet.ID).
			UpdateColumn("balance", gorm.Expr("balance + ?", intent.Amount.Minor())).Error
		if err != nil {
			return err
		}

		intent.Status = db.TransactionSuccess
		if err := tx.Model(&intent).Update("status", db.TransactionSuccess).Error; err != nil {
			return err
		}

		settled = t.MapIntentModelToIntent(&intent)
		return nil
	})
	if err != nil {
		return nil, ctx.LogError("transaction.settle", err, slog.String("reference", param.Reference))
	}
	return settled, nil
}
