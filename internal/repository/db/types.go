package db

import (
	"time"

	"github.com/google/uuid"
	"github/marveldo/eda-monolith/shared"
	"gorm.io/gorm"
)

type Gender string

const (
	GenderMale   Gender = "MALE"
	GenderFemale Gender = "FEMALE"
	GenderOther  Gender = "OTHER"
)

type WalletStatus string

const (
	WalletActive WalletStatus = "ACTIVE"
	WallteFrozen WalletStatus = "FROZEN"
	WalletClosed WalletStatus = "CLOSED"
)

type TransactionStatus string

const (
	TransactionPending TransactionStatus = "PENDING"
	TransactionSuccess TransactionStatus = "SUCCESS"
	TransactionFailed  TransactionStatus = "FAILED"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
	CurrencyJPY Currency = "JPY"
	CurrencyAUD Currency = "AUD"
	CurrencyCAD Currency = "CAD"
	CurrencyCHF Currency = "CHF"
	CurrencyCNY Currency = "CNY"
	CurrencySEK Currency = "SEK"
	CurrencyNZD Currency = "NZD"
	CurrencyNGN Currency = "NGN"
)

type User struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey"`
	FirstName       string     `gorm:"type:varchar(100);not null"`
	LastName        string     `gorm:"type:varchar(100);not null"`
	Email           string     `gorm:"type:varchar(100);unique;not null"`
	Passwordhash    *string    `gorm:"type:varchar(255);not null"`
	PhoneNumber     *string    `gorm:"type:varchar(20)"`
	DateOfBirth     *time.Time `gorm:"type:date"`
	ProfilePhotoURL *string
	Gender          Gender `gorm:"type:varchar(10);not null;default:'OTHER'"`
	Wallets         []Wallet
	Activities      []UserActivity
	IsActive        bool `gorm:"type:boolean;not null;default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
	IsVerified      bool           `gorm:"type:boolean;not null;default:false"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type Wallet struct {
	ID        uuid.UUID    `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID    `gorm:"type:uuid;not null;index"`
	Currency  Currency     `gorm:"type:varchar(3);not null;default:'USD'"`
	Status    WalletStatus `gorm:"type:varchar(10); not null;default:'ACTIVE'"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (w *Wallet) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

type UserActivity struct {
	gorm.Model
	UserID uuid.UUID `gorm:"type:uuid;not null;index"`
	Action string    `gorm:"type:text;not null;"`
}

type TransactionIntent struct {
	ID        uuid.UUID         `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID         `gorm:"type:uuid;not null;index"`
	Amount    shared.Money      `gorm:"type:bigint;not null"`
	Status    TransactionStatus `gorm:"type:text;not null;default:'PENDING'"`
	Wallet    uuid.UUID         `gorm:"type:uuid;not null;index"`
	Provider  string            `gorm:"type:text;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *TransactionIntent) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}
