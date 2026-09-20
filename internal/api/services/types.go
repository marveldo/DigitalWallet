package services

import (
	"time"

	"github/marveldo/eda-monolith/shared"
)

type UserInputParam struct {
	FirstName       string
	LastName        string
	Email           string
	Password        string
	PhoneNumber     *string
	DateOfBirth     *string
	ProfilePhotoURL *string
	Gender          *string
}
type UpdateUserParam struct {
	FirstName       *string
	LastName        *string
	PhoneNumber     *string
	ProfilePhotoURL *string
	Gender          *string
}

type User struct {
	ID              string
	FirstName       string
	LastName        string
	Email           string
	PhoneNumber     *string
	DateOfBirth     *time.Time
	ProfilePhotoURL *string
	Gender          string
	IsVerified      bool
}

type LoginSuccessful struct {
	AccessToken  string
	RefreshToken string
	User         User
}

type ResendOtpOk struct {
	Message string
}

type AccountVerificationOk struct {
	Message string
}

type Activity struct {
	ID        uint
	UserID    string
	Action    string
	CreatedAt string
}

// ActivityPageParam is the raw, untrusted paging request. Normalise clamps it
// into something safe to send to the database.
type ActivityPageParam struct {
	Limit  *int
	Offset *int
}

func (p *ActivityPageParam) Normalise() (limit int, offset int) {
	limit = DefaultActivityLimit
	if p != nil && p.Limit != nil {
		limit = *p.Limit
	}
	if limit < 1 {
		limit = DefaultActivityLimit
	}
	if limit > MaxActivityLimit {
		limit = MaxActivityLimit
	}
	if p != nil && p.Offset != nil && *p.Offset > 0 {
		offset = *p.Offset
	}
	return limit, offset
}

type ActivityList struct {
	Activities []Activity
	Total      int64
	Limit      int
	Offset     int
}

const TimeLayout = time.RFC3339

type InitializeDepositParam struct {
	Amount   float64
	WalletID string
}

func (p *InitializeDepositParam) Money() shared.Money {
	return shared.NewMoney(p.Amount)
}

type Wallet struct {
	ID            string
	AccountNumber string
	Currency      string
	Status        string
	Balance       shared.Money
}

type Transfer struct {
	ID                string
	SenderWalletID    string
	RecipientWalletID string
	Amount            shared.Money
	Currency          string
	Status            string
	FailureReason     string
	CreatedAt         string
}

type InitiateTransferParam struct {
	SenderWalletID         string
	RecipientAccountNumber string
	Amount                 float64
}

func (p *InitiateTransferParam) Money() shared.Money {
	return shared.NewMoney(p.Amount)
}

type DepositInitialized struct {
	Reference        string
	AuthorizationURL string
	AccessCode       string
	Amount           shared.Money
	Currency         string
	Status           string
}

// TransactionKind separates a deposit from a wallet-to-wallet transfer. Both
// look the same in the ledger, so the kind is what tells them apart.
type TransactionKind string

const (
	TransactionKindDeposit  TransactionKind = "DEPOSIT"
	TransactionKindTransfer TransactionKind = "TRANSFER"
)

type TransactionStatus string

const TransactionStatusSuccess TransactionStatus = "SUCCESS"

type Transaction struct {
	Reference string
	UserID    string
	WalletID  string
	Amount    shared.Money
	Status    string
	Provider  string
	Kind      TransactionKind
	// Direction is relative to the caller: CREDIT is money arriving in their
	// wallet, DEBIT is money leaving it.
	Direction string
	CreatedAt string
}

type TransactionList struct {
	Transactions []Transaction
	Total        int64
	Limit        int
	Offset       int
}
