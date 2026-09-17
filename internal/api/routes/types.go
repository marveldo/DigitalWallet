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

type CreateWalletRequest struct {
	Currency string `json:"currency,omitempty" validate:"omitempty,len=3" example:"NGN"`
}

type WalletResponse struct {
	ID       string  `json:"id"`
	Currency string  `json:"currency" example:"NGN"`
	Status   string  `json:"status" example:"ACTIVE"`
	Balance  float64 `json:"balance" example:"5000.00"`
}

type WalletListResponse struct {
	Wallets []WalletResponse `json:"wallets"`
}

type InitializeDepositRequest struct {
	WalletID string  `json:"wallet_id" validate:"required,uuid" example:"3f6c1a52-8d2e-4b7a-9c41-2f0e5d8b7a13"`
	Amount   float64 `json:"amount" validate:"required,gt=0" example:"5000.00"`
}

type DepositResponse struct {
	Reference        string  `json:"reference"`
	AuthorizationURL string  `json:"authorization_url" description:"Send the user here to complete the payment"`
	AccessCode       string  `json:"access_code"`
	Amount           float64 `json:"amount" example:"5000.00"`
	Currency         string  `json:"currency" example:"NGN"`
	Status           string  `json:"status" example:"PENDING"`
}

type TransactionResponse struct {
	Reference string  `json:"reference"`
	WalletID  string  `json:"wallet_id"`
	Amount    float64 `json:"amount" example:"5000.00"`
	Status    string  `json:"status" example:"SUCCESS"`
	Provider  string  `json:"provider" example:"paystack"`
	CreatedAt string  `json:"created_at" example:"2026-09-12T14:30:00Z"`
}

type TransactionListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Total        int64                 `json:"total"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
}
