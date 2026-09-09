package repository

import "time"

type User struct {
	ID              string
	FirstName       string
	LastName        string
	Email           string
	PhoneNumber     *string
	DateOfBirth     *time.Time
	ProfilePhotoURL *string
	Gender          string
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
	Balance  float64
	Currency string
}

type UserInputParam struct {
	FirstName       string
	LastName        string
	Email           string
	Passwordhash    *string
	PhoneNumber     *string
	DateOfBirth     *time.Time
	ProfilePhotoURL *string
	Gender          *string
}
