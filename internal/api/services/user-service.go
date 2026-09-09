package services

import (
	"log/slog"
	"net/http"
	"time"

	"github/marveldo/eda-monolith/internal/repository"
	"github/marveldo/eda-monolith/shared"
)

func (s *Service) MapUserToServiceDomain(user *repository.User) *User {
	return &User{
		ID:              user.ID,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Email:           user.Email,
		DateOfBirth:     user.DateOfBirth,
		PhoneNumber:     user.PhoneNumber,
		ProfilePhotoURL: user.ProfilePhotoURL,
		Gender:          user.Gender,
	}
}

func (s *Service) CreateUser(ctx *ServiceCtx, userInput *UserInputParam) (*User, *shared.AppError) {
	log := ctx.Logger.With(slog.String("service", "user.create"), slog.String("email", userInput.Email))
	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: ctx.Span, Logger: ctx.Logger})
	exists, err := s.Repository.UserEmailExists(repo_ctx, userInput.Email)
	var dob time.Time
	layout := "2006-01-02 15:04:00"
	if err != nil {
		log.Error("could not check whether email is taken", slog.Any("error", err))
		return nil, &shared.AppError{Message: err.Error(), Err: err, Code: http.StatusInternalServerError}
	}
	if exists {
		log.Warn("registration rejected, email already taken")
		return nil, &shared.AppError{Message: "User With This Email Already Exists", Err: err, Code: http.StatusConflict}
	}
	hashed_password, err := HashPassword([]byte(userInput.Password))
	if err != nil {
		log.Error("could not hash password", slog.Any("error", err))
		return nil, &shared.AppError{Message: "Error Registering User", Err: err, Code: http.StatusInternalServerError}
	}
	if userInput.DateOfBirth != nil {
		obj, err := time.Parse(layout, *userInput.DateOfBirth)
		if err != nil {
			log.Warn("could not parse date of birth", slog.String("layout", layout), slog.Any("error", err))
			return nil, &shared.AppError{Message: "Error Registering User", Err: err, Code: http.StatusInternalServerError}
		}
		dob = obj
	}
	stringfiedHash := string(hashed_password)
	user, err := s.Repository.CreateUser(repo_ctx, &repository.UserInputParam{
		FirstName:       userInput.FirstName,
		LastName:        userInput.LastName,
		Email:           userInput.Email,
		PhoneNumber:     userInput.PhoneNumber,
		DateOfBirth:     &dob,
		ProfilePhotoURL: userInput.ProfilePhotoURL,
		Gender:          userInput.Gender,
		Passwordhash:    &stringfiedHash,
	})
	if err != nil {
		log.Error("could not persist user", slog.Any("error", err))
		return nil, &shared.AppError{Message: "Error Registering User", Err: err, Code: http.StatusInternalServerError}
	}
	log.Info("user created", slog.String("user_id", user.ID))
	return s.MapUserToServiceDomain(user), nil
}
