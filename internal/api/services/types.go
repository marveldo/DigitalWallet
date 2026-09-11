package services

import "time"

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
