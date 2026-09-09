package db

import (
	"time"

	"gorm.io/gorm"
)

type Gender string

const (
   GenderMale  Gender = "MALE";
   GenderFemale Gender = "FEMALE";
   GenderOther  Gender = "OTHER";
)

type Currency string 

const (
   CurrencyUSD Currency = "USD";
   CurrencyEUR Currency = "EUR";
   CurrencyGBP Currency = "GBP";
   CurrencyJPY Currency = "JPY";
   CurrencyAUD Currency = "AUD";
   CurrencyCAD Currency = "CAD";
   CurrencyCHF Currency = "CHF";
   CurrencyCNY Currency = "CNY";
   CurrencySEK Currency = "SEK";
   CurrencyNZD Currency = "NZD";
   CurrencyNGN Currency = "NGN";

)

type User struct {
	gorm.Model
	ID   string `gorm:"type:varchar(36);primaryKey;default:UUID()"` 
	FirstName string `gorm:"type:varchar(100);not null"`
	LastName  string `gorm:"type:varchar(100);not null"`
	Email    string `gorm:"type:varchar(100);unique;not null"`
	Passwordhash  *string `gorm:"type:varchar(255);not null"`
	PhoneNumber  *string `gorm:"type:varchar(20);unique"`
	DateOfBirth  *time.Time `gorm:"type:date"`
	ProfilePhotoURL *string
	Gender Gender `gorm:"type:varchar(10);not null;default:'OTHER'"`
	Wallets []Wallet 
	IsActive bool `gorm:"type:boolean;not null;default:true"`
}

type Wallet struct {
	gorm.Model
	ID        string `gorm:"type:varchar(36);primaryKey;default:UUID()"`
	UserID    string `gorm:"type:varchar(36);not null"`
	Balance   float64 `gorm:"type:decimal(10,2);not null;default:0.00"`
	Currency  Currency `gorm:"type:varchar(3);not null;default:'USD'"`
}


