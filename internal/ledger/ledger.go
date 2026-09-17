package ledger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"

	"github/marveldo/eda-monolith/shared"

	"github.com/google/uuid"
	tb "github.com/tigerbeetle/tigerbeetle-go"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ErrRejected = errors.New("ledger rejected the request")

type Ledger struct {
	Client tb.Client
	trace.Tracer
	Logger *slog.Logger
}

type LedgerConfig struct {
	ClusterID uint64
	Addresses []string
	trace.Tracer
	Logger *slog.Logger
}

type DepositParam struct {
	TransferID uuid.UUID
	WalletID   uuid.UUID
	Currency   string
	Amount     shared.Money
}

func NewLedger(cfg *LedgerConfig) (*Ledger, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("layer", "ledger"))

	client, err := tb.NewClient(tb.ToUint128(cfg.ClusterID), cfg.Addresses)
	if err != nil {
		return nil, fmt.Errorf("tigerbeetle client: %w", err)
	}
	return &Ledger{
		Client: client,
		Tracer: cfg.Tracer,
		Logger: logger,
	}, nil
}

func (l *Ledger) Close() {
	if l.Client != nil {
		l.Client.Close()
	}
}

func (l *Ledger) StartSpan(ctx context.Context, op string) (context.Context, trace.Span) {
	if l.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return l.Tracer.Start(ctx, "ledger."+op)
}

func AccountID(walletID uuid.UUID) tb.Uint128 {
	return tb.BytesToUint128(walletID)
}

func TransferID(intentID uuid.UUID) tb.Uint128 {
	return tb.BytesToUint128(intentID)
}
func SettlementAccountID(ledgerID LedgerID) tb.Uint128 {
	return tb.ToUint128(uint64(ledgerID))
}

func (l *Ledger) Deposit(ctx context.Context, param DepositParam) error {
	ctx, span := l.StartSpan(ctx, "deposit")
	defer span.End()

	err := l.deposit(param)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		l.Logger.ErrorContext(ctx, "ledger deposit failed",
			slog.String("transfer_id", param.TransferID.String()),
			slog.String("wallet_id", param.WalletID.String()),
			slog.Any("error", err),
		)
	}
	return err
}

func (l *Ledger) deposit(param DepositParam) error {
	if !param.Amount.IsPositive() {
		return fmt.Errorf("%w: deposit amount must be positive", ErrRejected)
	}
	ledgerID, err := LedgerForCurrency(param.Currency)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRejected, err)
	}

	if err := l.ensureAccounts(ledgerID, param.WalletID); err != nil {
		return err
	}

	results, err := l.Client.CreateTransfers([]tb.Transfer{{
		ID:              TransferID(param.TransferID),
		DebitAccountID:  SettlementAccountID(ledgerID),
		CreditAccountID: AccountID(param.WalletID),
		Amount:          tb.ToUint128(uint64(param.Amount.Minor())),
		Ledger:          uint32(ledgerID),
		Code:            uint16(TransferDeposit),
	}})
	if err != nil {
		return fmt.Errorf("create transfer: %w", err)
	}
	for _, result := range results {
		switch result.Status {
		case tb.TransferCreated, tb.TransferExists:
		default:
			return fmt.Errorf("%w: create transfer: %s", ErrRejected, result.Status)
		}
	}
	return nil
}

func (l *Ledger) TransferExists(ctx context.Context, intentID uuid.UUID) (bool, error) {
	ctx, span := l.StartSpan(ctx, "transfer_exists")
	defer span.End()

	transfers, err := l.Client.LookupTransfers([]tb.Uint128{TransferID(intentID)})
	if err != nil {
		err = fmt.Errorf("lookup transfers: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		l.Logger.ErrorContext(ctx, "ledger transfer lookup failed",
			slog.String("transfer_id", intentID.String()), slog.Any("error", err))
		return false, err
	}
	return len(transfers) > 0, nil
}

func (l *Ledger) EnsureWalletAccount(ctx context.Context, walletID uuid.UUID, currency string) error {
	ctx, span := l.StartSpan(ctx, "ensure_wallet_account")
	defer span.End()

	err := func() error {
		ledgerID, err := LedgerForCurrency(currency)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRejected, err)
		}
		return l.ensureAccounts(ledgerID, walletID)
	}()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		l.Logger.ErrorContext(ctx, "ledger ensure wallet account failed",
			slog.String("wallet_id", walletID.String()),
			slog.String("currency", currency),
			slog.Any("error", err),
		)
	}
	return err
}


func (l *Ledger) EnsureSettlementAccounts(ctx context.Context) error {
	ctx, span := l.StartSpan(ctx, "ensure_settlement_accounts")
	defer span.End()

	accounts := make([]tb.Account, 0, len(currencyLedgers))
	for _, ledgerID := range currencyLedgers {
		accounts = append(accounts, tb.Account{
			ID:     SettlementAccountID(ledgerID),
			Ledger: uint32(ledgerID),
			Code:   uint16(AccountSettlement),
		})
	}
	err := l.createAccounts(accounts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		l.Logger.ErrorContext(ctx, "ledger ensure settlement accounts failed", slog.Any("error", err))
	}
	return err
}


func (l *Ledger) Balances(ctx context.Context, walletIDs []uuid.UUID) (map[uuid.UUID]shared.Money, error) {
	ctx, span := l.StartSpan(ctx, "balances")
	defer span.End()

	balances := make(map[uuid.UUID]shared.Money, len(walletIDs))
	if len(walletIDs) == 0 {
		return balances, nil
	}
	ids := make([]tb.Uint128, 0, len(walletIDs))
	byAccount := make(map[tb.Uint128]uuid.UUID, len(walletIDs))
	for _, walletID := range walletIDs {
		balances[walletID] = 0
		id := AccountID(walletID)
		ids = append(ids, id)
		byAccount[id] = walletID
	}

	accounts, err := l.Client.LookupAccounts(ids)
	if err != nil {
		err = fmt.Errorf("lookup accounts: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		l.Logger.ErrorContext(ctx, "ledger balance lookup failed", slog.Any("error", err))
		return nil, err
	}
	for _, account := range accounts {
		credits := account.CreditsPosted.BigInt()
		debits := account.DebitsPosted.BigInt()
		balance := new(big.Int).Sub(credits, debits)
		balances[byAccount[account.ID]] = shared.NewMoneyFromMinor(balance.Int64())
	}
	return balances, nil
}

func (l *Ledger) ensureAccounts(ledgerID LedgerID, walletID uuid.UUID) error {
	return l.createAccounts([]tb.Account{
		{
			ID:     SettlementAccountID(ledgerID),
			Ledger: uint32(ledgerID),
			Code:   uint16(AccountSettlement),
		},
		{
			ID:     AccountID(walletID),
			Ledger: uint32(ledgerID),
			Code:   uint16(AccountWallet),
			Flags:  tb.AccountFlags{DebitsMustNotExceedCredits: true}.ToUint16(),
		},
	})
}

func (l *Ledger) createAccounts(accounts []tb.Account) error {
	results, err := l.Client.CreateAccounts(accounts)
	if err != nil {
		return fmt.Errorf("create accounts: %w", err)
	}
	for _, result := range results {
		switch result.Status {
		case tb.AccountCreated, tb.AccountExists:
		default:
			return fmt.Errorf("%w: create account: %s", ErrRejected, result.Status)
		}
	}
	return nil
}
