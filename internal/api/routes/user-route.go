package routes

import (
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/internal/api/services"

	"github.com/oaswrap/spec"
	"github.com/oaswrap/spec/option"
)

func UserDocs(r spec.Router) {
	r.Post("/api/v1/users",
		option.OperationID("user-create"),
		option.Summary("Create a user"),
		option.Description("Registers a new user and returns the created record."),
		option.Tags("Users"),
		option.Request(new(CreateUserRequest)),
		option.Response(http.StatusCreated, new(UserResponse)),
		option.Response(http.StatusConflict, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
	)
}

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

	user, appErr := rt.Services.CreateUser(serviceCtx, &services.UserInputParam{
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

	log.Info("user registered", slog.String("user_id", user.ID))
	rt.WriteJSON(w, http.StatusCreated, rt.MapUserToRouteDomain(user))
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
	}
}
