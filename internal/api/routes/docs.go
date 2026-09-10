package routes

import (
	"github.com/oaswrap/spec"
	"github.com/oaswrap/spec/option"
	"net/http"
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

	r.Patch("/api/v1/users/{id}",
		option.OperationID("user-update"),
		option.Summary("Update a user"),
		option.Description("Updates the supplied fields on a user. Only the fields present in the body are changed."),
		option.Tags("Users"),
		option.Request(new(UpdateUserRequest)),
		option.Response(http.StatusOK, new(UserResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusConflict, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
	)

	r.Delete("/api/v1/users/{id}",
		option.OperationID("user-delete"),
		option.Summary("Delete a user"),
		option.Description("Deactivates a user account."),
		option.Tags("Users"),
		option.Response(http.StatusNoContent, nil),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
	)

	r.Post("/api/v1/users/otp/resend",
		option.OperationID("user-otp-resend"),
		option.Summary("Resend a verification OTP"),
		option.Description("Issues a fresh OTP and emails it. Always reports success so the endpoint cannot be used to discover which emails are registered."),
		option.Tags("Users"),
		option.Request(new(ResendOtpRequest)),
		option.Response(http.StatusOK, new(MessageResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)

	r.Post("/api/v1/users/otp/verify",
		option.OperationID("user-otp-verify"),
		option.Summary("Verify an account with an OTP"),
		option.Description("Confirms the emailed code and marks the account verified. The code is single use."),
		option.Tags("Users"),
		option.Request(new(VerifyOtpRequest)),
		option.Response(http.StatusOK, new(MessageResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)
}