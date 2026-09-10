package services

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github/marveldo/eda-monolith/internal/api/events"
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
		IsVerified:      user.IsVerified,
	}
}

func (s *Service) CreateUser(ctx *ServiceCtx, userInput *UserInputParam) (*User, *shared.AppError) {
	ctx, span := ctx.Start("user.create")
	defer span.End()

	log := ctx.Logger.With(slog.String("service", "user.create"), slog.String("email", userInput.Email))

	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: ctx.Span, Logger: ctx.Logger})
	exists, err := s.Repository.UserEmailExists(repo_ctx, userInput.Email)
	var dob time.Time
	layout := "2006-01-02 15:04:00"
	if err != nil {
		log.Error("could not check whether email is taken", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: err.Error(), Err: err, Code: http.StatusInternalServerError})
	}
	if exists {
		log.Warn("registration rejected, email already taken")
		return nil, ctx.Fail(&shared.AppError{Message: "User With This Email Already Exists", Err: err, Code: http.StatusConflict})
	}
	hashed_password, err := HashPassword([]byte(userInput.Password))
	if err != nil {
		log.Error("could not hash password", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Error Registering User", Err: err, Code: http.StatusInternalServerError})
	}
	if userInput.DateOfBirth != nil {
		obj, err := time.Parse(layout, *userInput.DateOfBirth)
		if err != nil {
			log.Warn("could not parse date of birth", slog.String("layout", layout), slog.Any("error", err))
			return nil, ctx.Fail(&shared.AppError{Message: "Error Registering User", Err: err, Code: http.StatusInternalServerError})
		}
		dob = obj
	}
	stringfiedHash := string(hashed_password)
	user, err := s.Repository.CreateUser(repo_ctx, &repository.UserInputParam{
		FirstName:       shared.Ptr(userInput.FirstName),
		LastName:        shared.Ptr(userInput.LastName),
		Email:           shared.Ptr(userInput.Email),
		PhoneNumber:     userInput.PhoneNumber,
		DateOfBirth:     shared.Ptr(dob),
		ProfilePhotoURL: userInput.ProfilePhotoURL,
		Gender:          userInput.Gender,
		Passwordhash:    shared.Ptr(stringfiedHash),
	})
	if err != nil {
		log.Error("could not persist user", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Message: "Error Registering User", Err: err, Code: http.StatusInternalServerError})
	}
	log.Info("user created", slog.String("user_id", user.ID))

	s.PublishEvent(ctx, events.EventUserCreated, events.UserCreatedPayload{
		UserID:    user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		CreatedAt: time.Now().UTC(),
	})

	return s.MapUserToServiceDomain(user), nil
}

func (s *Service) ResendUserOtp(ctx *ServiceCtx, email string) (*ResendOtpOk, *shared.AppError) {
	ctx, span := ctx.Start("resend.otp")
	defer span.End()

	log := ctx.Logger.With(slog.String("service", "request.otp"), slog.String("input", email))

	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	exists, err := s.Repository.UserEmailExists(repo_ctx, email)
	if err != nil {
		log.Error("Could not Check Whther User Exists", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Err: err, Message: "Couldnt Verify User", Code: http.StatusInternalServerError})
	}
	if !exists {
		log.Warn("Account With This Email Doesnt Exist")
		return &ResendOtpOk{
			Message: "Otp Resent You Should Recieve it If Your Account Exists",
		}, nil
	}
	user, err := s.Repository.GetUserByEmail(repo_ctx, email)
	if err != nil {
		log.Error("Could not Retrive User Account", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Err: err, Message: "Couldnt Verify User", Code: http.StatusInternalServerError})
	}
	s.PublishEvent(ctx, events.EventUserCreated, events.UserCreatedPayload{
		UserID:    user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		CreatedAt: time.Now().UTC(),
	})
	log.Info("otp.resent", slog.String("user_id", user.ID))
	return &ResendOtpOk{
		Message: "Otp Resent You Should Recieve it If Your Account Exists",
	}, nil
}

func (s *Service) VerifyOtp(ctx *ServiceCtx, email string, otpInput string) (*AccountVerificationOk, *shared.AppError) {
	ctx, span := ctx.Start("verify.otp")
	defer span.End()

	log := ctx.Logger.With(slog.String("service", "request.otp"), slog.String("input", email))

	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: s.Logger})
	exists, err := s.Repository.UserEmailExists(repo_ctx, email)
	if err != nil {
		log.Error("Could not Check Whether User Exists", slog.Any("error", err))
		return nil, ctx.Fail(&shared.AppError{Err: err, Message: "Couldnt Verify User", Code: http.StatusInternalServerError})
	}
	if !exists {
		log.Warn("Account With This Email Doesnt Exist")
		return nil, ctx.Fail(&shared.AppError{Err: err, Message: "Couldnt Verify User", Code: http.StatusBadRequest})
	}
	err = s.Store.Verify(ctx.Context, email, otpInput)
	if err != nil {
		log.Warn("Otp MisMatch")
		return nil, ctx.Fail(&shared.AppError{Err: err, Message: "Couldnt Verify User", Code: http.StatusBadRequest})
	}
	_, err = s.Repository.UpdateUser(repo_ctx, &repository.UserInputParam{
		Email:      shared.Ptr(email),
		IsVerified: shared.Ptr(true),
	})

	log.Info("otp.Verified", slog.String("user_email", email))

	return &AccountVerificationOk{
		Message: "Account Verified Successfully",
	}, nil

}

// TODO(login): unfinished — placeholder body so the package compiles.
func (s *Service) LoginUser(ctx *ServiceCtx, email string, password string) {
}
func (s *Service) UpdateUser(ctx *ServiceCtx, id string, input *UpdateUserParam) (*User, *shared.AppError) {
	ctx, span := ctx.Start("user.update")
	defer span.End()

	log := ctx.Logger.With(slog.String("service", "user.update"), slog.String("user_id", id))
	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	user, err := s.Repository.UpdateUser(repo_ctx, &repository.UserInputParam{
		ID:              &id,
		FirstName:       input.FirstName,
		LastName:        input.LastName,
		PhoneNumber:     input.PhoneNumber,
		ProfilePhotoURL: input.ProfilePhotoURL,
		Gender:          input.Gender,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			log.Warn("user not found")
			return nil, ctx.Fail(&shared.AppError{Message: "User Not Found", Err: err, Code: http.StatusNotFound})
		case errors.Is(err, repository.ErrNothingToUpdate):
			log.Warn("update requested with no fields")
			return nil, ctx.Fail(&shared.AppError{Message: "No Fields To Update", Err: err, Code: http.StatusBadRequest})
		case errors.Is(err, repository.ErrAlreadyExists):
			log.Warn("update conflicts with an existing record")
			return nil, ctx.Fail(&shared.AppError{Message: "Phone Number Already In Use", Err: err, Code: http.StatusConflict})
		default:
			log.Error("could not update user", slog.Any("error", err))
			return nil, ctx.Fail(&shared.AppError{Message: "Could Not Update User", Err: err, Code: http.StatusInternalServerError})
		}
	}

	log.Info("user updated")
	return s.MapUserToServiceDomain(user), nil
}

func (s *Service) DeleteUser(ctx *ServiceCtx, id string) *shared.AppError {
	ctx, span := ctx.Start("user.delete")
	defer span.End()

	log := ctx.Logger.With(slog.String("service", "user.delete"), slog.String("user_id", id))
	repo_ctx := s.GetRepoCtx(ServiceCtxConfig{Context: ctx.Context, Span: span, Logger: ctx.Logger})

	if err := s.Repository.DeleteUser(repo_ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			log.Warn("user not found")
			return ctx.Fail(&shared.AppError{Message: "User Not Found", Err: err, Code: http.StatusNotFound})
		}
		log.Error("could not delete user", slog.Any("error", err))
		return ctx.Fail(&shared.AppError{Message: "Could Not Delete User", Err: err, Code: http.StatusInternalServerError})
	}

	log.Info("user deleted")
	return nil
}
