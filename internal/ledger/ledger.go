package ledger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github/marveldo/eda-monolith/shared"

	"github.com/google/uuid"
	tb "github.com/tigerbeetle/tigerbeetle-go"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ErrRejected means TigerBeetle refused the request outright. Retrying the same
// request will be refused again, so callers should not retry it.
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

// SettlementAccountID is the per-currency account deposits are debited from.
// It stands for money held at the payment provider, so it runs a debit
// balance. Small integers never collide with wallet ids, which are UUIDv4.
func SettlementAccountID(ledgerID LedgerID) tb.Uint128 {
	return tb.ToUint128(uint64(ledgerID))
}

// Deposit moves amount from the currency's settlement account into the wallet.
// The transfer id comes from the caller, so repeating a deposit is a no-op:
// TigerBeetle answers TransferExists and nothing is credited twice.
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

// ensureAccounts creates the settlement and wallet accounts if they do not
// exist yet. Creating an identical account again answers AccountExists.
func (l *Ledger) ensureAccounts(ledgerID LedgerID, walletID uuid.UUID) error {
	results, err := l.Client.CreateAccounts([]tb.Account{
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
