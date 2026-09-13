package repository

import (
	"time"

	"github/marveldo/eda-monolith/shared"
)

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
	IsActive        bool
}

type UserFilters struct {
	FirstName *string
	LastName  *string
	Email     *string
	Gender    *string
}

type Wallet struct {
	ID       string
	UserID   string
	Balance  shared.Money
	Currency string
}

type UserInputParam struct {
	ID              *string
	FirstName       *string
	LastName        *string
	Email           *string
	Passwordhash    *string
	PhoneNumber     *string
	DateOfBirth     *time.Time
	ProfilePhotoURL *string
	Gender          *string
	IsVerified      *bool
}

type Activity struct {
	Id        uint
	UserID    string
	CreatedAt string
	Action    string
}

type TransactionIntent struct {
	ID        string
	UserID    string
	WalletID  string
	Amount    shared.Money
	Status    string
	Provider  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateIntentParam struct {
	UserID   string
	WalletID string
	Amount   shared.Money
	Provider string
}

type SettleIntentParam struct {
	Reference   string
	Amount      shared.Money
	Description string
}

type IntentPage struct {
	Intents []*TransactionIntent
	Total   int64
}
