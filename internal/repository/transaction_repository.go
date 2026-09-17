package repository

import (
	"errors"
	"log/slog"

	"github/marveldo/eda-monolith/internal/repository/db"

	"gorm.io/gorm/clause"
)

var (
	ErrIntentNotPending = errors.New("transaction intent is no longer pending")
	ErrIntentSucceeded  = errors.New("transaction intent has already succeeded")
	ErrIntentNotFailed  = errors.New("transaction intent is not in the expected refund state")
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
		Currency: string(model.Currency),
		Status:   string(model.Status),
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

func (t *TransactionRepository) ListWalletsByUser(ctx *RepoCtx, userID string) ([]*Wallet, error) {
	ctx, span := ctx.Start("transaction.list_wallets_by_user")
	defer span.End()

	parsedUser, err := ParseUUID(userID)
	if err != nil {
		return nil, ctx.LogError("transaction.list_wallets_by_user", err, slog.String("user_id", userID))
	}

	var models []db.Wallet
	if err := ctx.DB.WithContext(ctx.Context).Where("user_id = ?", parsedUser).Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, ctx.LogError("transaction.list_wallets_by_user", err, slog.String("user_id", userID))
	}
	wallets := make([]*Wallet, 0, len(models))
	for i := range models {
		wallets = append(wallets, t.MapWalletModelToWallet(&models[i]))
	}
	return wallets, nil
}

func (t *TransactionRepository) CreateWallet(ctx *RepoCtx, userID string, currency string) (*Wallet, error) {
	ctx, span := ctx.Start("transaction.create_wallet")
	defer span.End()

	parsedUser, err := ParseUUID(userID)
	if err != nil {
		return nil, ctx.LogError("transaction.create_wallet", err, slog.String("user_id", userID))
	}

	wallet := db.Wallet{UserID: parsedUser, Currency: db.Currency(currency)}
	if err := ctx.DB.WithContext(ctx.Context).Create(&wallet).Error; err != nil {
		return nil, ctx.LogError("transaction.create_wallet", err, slog.String("user_id", userID), slog.String("currency", currency))
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

// MarkIntentRefunding claims a failed intent for a refund. It also accepts an
// intent already REFUNDING, so a retried refund task can pick it up again, but
// never one that is PENDING, SUCCESS or REFUNDED.
func (t *TransactionRepository) MarkIntentRefunding(ctx *RepoCtx, reference string) (*TransactionIntent, error) {
	return t.moveIntent(ctx, "transaction.mark_refunding", reference,
		[]db.TransactionStatus{db.TransactionFailed, db.TransactionRefunding}, db.TransactionRefunding)
}

// MarkIntentRefunded closes an intent once the provider accepted the refund.
func (t *TransactionRepository) MarkIntentRefunded(ctx *RepoCtx, reference string) (*TransactionIntent, error) {
	return t.moveIntent(ctx, "transaction.mark_refunded", reference,
		[]db.TransactionStatus{db.TransactionRefunding}, db.TransactionRefunded)
}

func (t *TransactionRepository) moveIntent(ctx *RepoCtx, op string, reference string, from []db.TransactionStatus, to db.TransactionStatus) (*TransactionIntent, error) {
	ctx, span := ctx.Start(op)
	defer span.End()

	parsed, err := ParseUUID(reference)
	if err != nil {
		return nil, ctx.LogError(op, err, slog.String("reference", reference))
	}

	var intent db.TransactionIntent
	result := ctx.DB.WithContext(ctx.Context).
		Model(&intent).
		Clauses(clause.Returning{}).
		Where("id = ? AND status IN ?", parsed, from).
		Update("status", to)
	if result.Error != nil {
		return nil, ctx.LogError(op, result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrIntentNotFailed
	}
	return t.MapIntentModelToIntent(&intent), nil
}

func (t *TransactionRepository) GetWalletByID(ctx *RepoCtx, walletID string) (*Wallet, error) {
	ctx, span := ctx.Start("transaction.wallet_by_id")
	defer span.End()

	parsed, err := ParseUUID(walletID)
	if err != nil {
		return nil, ctx.LogError("transaction.wallet_by_id", err, slog.String("wallet_id", walletID))
	}

	var wallet db.Wallet
	if err := ctx.DB.WithContext(ctx.Context).Where("id = ?", parsed).First(&wallet).Error; err != nil {
		return nil, ctx.LogError("transaction.wallet_by_id", err, slog.String("wallet_id", walletID))
	}
	return t.MapWalletModelToWallet(&wallet), nil
}

// MarkIntentSucceeded moves an intent to SUCCESS once the ledger holds the
// deposit. It is allowed from FAILED as well as PENDING: the provider confirmed
// the money arrived, and the ledger already credited it, so a poll that gave
// up earlier must not leave the intent saying otherwise.
func (t *TransactionRepository) MarkIntentSucceeded(ctx *RepoCtx, reference string) (*TransactionIntent, error) {
	ctx, span := ctx.Start("transaction.mark_succeeded")
	defer span.End()

	parsed, err := ParseUUID(reference)
	if err != nil {
		return nil, ctx.LogError("transaction.mark_succeeded", err, slog.String("reference", reference))
	}

	var intent db.TransactionIntent
	result := ctx.DB.WithContext(ctx.Context).
		Model(&intent).
		Clauses(clause.Returning{}).
		// FAILED is allowed: the ledger may have credited the wallet just
		// before the poller gave up on the intent.
		Where("id = ? AND status IN ?", parsed, []db.TransactionStatus{db.TransactionPending, db.TransactionFailed}).
		Update("status", db.TransactionSuccess)
	if result.Error != nil {
		return nil, ctx.LogError("transaction.mark_succeeded", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrIntentSucceeded
	}
	return t.MapIntentModelToIntent(&intent), nil
}
