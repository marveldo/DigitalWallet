package routes

import (
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/internal/api/services"

	"github.com/go-chi/chi/v5"
)

func (rt *Routes) CreateUser(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.create")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.create")
	if span != nil {
		defer span.End()
	}

	var body CreateUserRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	result, appErr := rt.Services.CreateUser(serviceCtx, &services.UserInputParam{
		FirstName:       body.FirstName,
		LastName:        body.LastName,
		Email:           body.Email,
		Password:        body.Password,
		PhoneNumber:     body.PhoneNumber,
		DateOfBirth:     body.DateOfBirth,
		ProfilePhotoURL: body.ProfilePhotoURL,
		Gender:          body.Gender,
	})
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("user registered", slog.String("user_id", result.User.ID))
	rt.WriteJSON(w, http.StatusCreated, LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		User:         rt.MapUserToRouteDomain(&result.User),
	})
}

func (rt *Routes) UpdateUser(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.update")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.update")
	if span != nil {
		defer span.End()
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Missing user id", nil)
		return
	}

	var body UpdateUserRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	user, appErr := rt.Services.UpdateUser(serviceCtx, id, &services.UpdateUserParam{
		FirstName:       body.FirstName,
		LastName:        body.LastName,
		PhoneNumber:     body.PhoneNumber,
		ProfilePhotoURL: body.ProfilePhotoURL,
		Gender:          body.Gender,
	})
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("user updated", slog.String("user_id", user.ID))
	rt.WriteJSON(w, http.StatusOK, rt.MapUserToRouteDomain(user))
}

func (rt *Routes) DeleteUser(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.delete")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.delete")
	if span != nil {
		defer span.End()
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		rt.WriteError(w, log, span, http.StatusBadRequest, "Missing user id", nil)
		return
	}

	if appErr := rt.Services.DeleteUser(serviceCtx, id); appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("user deleted", slog.String("user_id", id))
	rt.WriteJSON(w, http.StatusNoContent, nil)
}

func (rt *Routes) Login(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.login")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.login")
	if span != nil {
		defer span.End()
	}

	var body LoginRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	result, appErr := rt.Services.LoginUser(serviceCtx, body.Email, body.Password)
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("user logged in", slog.String("user_id", result.User.ID))
	rt.WriteJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		User:         rt.MapUserToRouteDomain(&result.User),
	})
}

func (rt *Routes) RefreshToken(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.refresh")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.refresh")
	if span != nil {
		defer span.End()
	}

	var body RefreshTokenRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	result, appErr := rt.Services.RefreshUserToken(serviceCtx, body.RefreshToken)
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("tokens refreshed", slog.String("user_id", result.User.ID))
	rt.WriteJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    "Bearer",
		User:         rt.MapUserToRouteDomain(&result.User),
	})
}

func (rt *Routes) ResendOtp(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.otp.resend")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.otp.resend")
	if span != nil {
		defer span.End()
	}

	var body ResendOtpRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	result, appErr := rt.Services.ResendUserOtp(serviceCtx, body.Email)
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("otp resend requested")
	rt.WriteJSON(w, http.StatusOK, MessageResponse{Message: result.Message})
}

func (rt *Routes) VerifyOtp(w http.ResponseWriter, r *http.Request) {
	log := rt.RequestLogger("user.otp.verify")
	serviceCtx, span := rt.GetServiceCtx(r.Context(), "user.otp.verify")
	if span != nil {
		defer span.End()
	}

	var body VerifyOtpRequest
	if !rt.DecodeAndValidate(w, r, log, span, &body) {
		return
	}

	result, appErr := rt.Services.VerifyOtp(serviceCtx, body.Email, body.Otp)
	if appErr != nil {
		rt.WriteAppError(w, log, span, appErr)
		return
	}

	log.Info("account verified", slog.String("email", body.Email))
	rt.WriteJSON(w, http.StatusOK, MessageResponse{Message: result.Message})
}

// MapUserToRouteDomain mirrors Service.MapUserToServiceDomain: each layer owns
// the translation into its own domain type.
func (rt *Routes) MapUserToRouteDomain(user *services.User) UserResponse {
	return UserResponse{
		ID:              user.ID,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Email:           user.Email,
		PhoneNumber:     user.PhoneNumber,
		DateOfBirth:     user.DateOfBirth,
		ProfilePhotoURL: user.ProfilePhotoURL,
		Gender:          user.Gender,
		IsVerified:      user.IsVerified,
	}
}
