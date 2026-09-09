package routes

import "time"

type UserResponse struct {
	ID              string     `json:"id"`
	FirstName       string     `json:"first_name"`
	LastName        string     `json:"last_name"`
	Email           string     `json:"email"`
	PhoneNumber     *string    `json:"phone_number,omitempty"`
	DateOfBirth     *time.Time `json:"date_of_birth,omitempty"`
	ProfilePhotoURL *string    `json:"profile_photo_url,omitempty"`
	Gender          string     `json:"gender,omitempty"`
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

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}
