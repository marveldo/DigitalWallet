package db

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Gender string

const (
	GenderMale   Gender = "MALE"
	GenderFemale Gender = "FEMALE"
	GenderOther  Gender = "OTHER"
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
	PhoneNumber     *string    `gorm:"type:varchar(20);unique"`
	DateOfBirth     *time.Time `gorm:"type:date"`
	ProfilePhotoURL *string
	Gender          Gender `gorm:"type:varchar(10);not null;default:'OTHER'"`
	Wallets         []Wallet
	IsActive        bool `gorm:"type:boolean;not null;default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate assigns the primary key in Go rather than leaning on a database
// default. The row's id is then known to the caller straight after Create,
// without a re-read, and the schema stays portable — gen_random_uuid() is
// Postgres-only (and UUID() is MySQL-only).
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type Wallet struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Balance   float64   `gorm:"type:decimal(10,2);not null;default:0.00"`
	Currency  Currency  `gorm:"type:varchar(3);not null;default:'USD'"`
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
