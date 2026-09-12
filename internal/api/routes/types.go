package routes

import "time"

type UserResponse struct {
	ID              string     `json:"id"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Email           string     `json:"email"`
	PhoneNumber     *string    `json:"phone_number"`
	DateOfBirth     *time.Time `json:"date_of_birth"`
	ProfilePhotoURL *string    `json:"profile_photo_url"`
	Gender          string     `json:"gender"`
	IsVerified      bool       `json:"is_verified"`
}

type CreateUserRequest struct {
	FirstName       string  `json:"first_name" validate:"required,max=100"`
	LastName        string  `json:"last_name" validate:"required,max=100"`
	Email           string  `json:"email" validate:"required,email"`
	Password        string  `json:"password" validate:"required,min=8,max=72"`
	PhoneNumber     *string `json:"phone_number,omitempty" validate:"omitempty,e164"`
	DateOfBirth     *string `json:"date_of_birth,omitempty" example:"1996-04-21 00:00:00"`
	ProfilePhotoURL *string `json:"profile_photo_url,omitempty" validate:"omitempty,url"`
	Gender          *string `json:"gender,omitempty" validate:"omitempty,oneof=MALE FEMALE OTHER"`
}

type ResendOtpRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type VerifyOtpRequest struct {
	Email string `json:"email" validate:"required,email"`
	Otp   string `json:"otp" validate:"required,numeric,min=4,max=10"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type" example:"Bearer"`
	User         UserResponse `json:"user"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type UpdateUserRequest struct {
	FirstName       *string `json:"first_name,omitempty" validate:"omitempty,max=100"`
	LastName        *string `json:"last_name,omitempty" validate:"omitempty,max=100"`
	PhoneNumber     *string `json:"phone_number,omitempty" validate:"omitempty,e164"`
	ProfilePhotoURL *string `json:"profile_photo_url,omitempty" validate:"omitempty,url"`
	Gender          *string `json:"gender,omitempty" validate:"omitempty,oneof=MALE FEMALE OTHER"`
}

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

type ActivityResponse struct {
	ID        uint   `json:"id"`
	UserID    string `json:"user_id"`
	Action    string `json:"action" example:"Logged Into Account"`
	CreatedAt string `json:"created_at" example:"2026-09-12T14:30:00Z"`
}

type ActivityListResponse struct {
	Activities []ActivityResponse `json:"activities"`
	Total      int64              `json:"total" description:"Total activities on the account, ignoring paging"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
}
