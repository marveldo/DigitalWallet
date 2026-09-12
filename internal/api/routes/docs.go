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
		option.Description("Updates the supplied fields on a user. Only the fields present in the body are changed. A caller may only update their own account."),
		option.Tags("Users"),
		option.Security(BearerSecurityScheme),
		option.Request(new(UpdateUserRequest)),
		option.Response(http.StatusOK, new(UserResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusForbidden, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusConflict, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
	)

	r.Delete("/api/v1/users/{id}",
		option.OperationID("user-delete"),
		option.Summary("Delete a user"),
		option.Description("Deactivates a user account. A caller may only delete their own account."),
		option.Tags("Users"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusNoContent, nil),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusForbidden, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
	)

	r.Post("/api/v1/users/login",
		option.OperationID("user-login"),
		option.Summary("Log a user in"),
		option.Description("Exchanges an email and password for an access/refresh token pair and the user record. A wrong email and a wrong password are reported identically, so the endpoint cannot be used to discover which addresses are registered. The account must already be verified."),
		option.Tags("Users"),
		option.Request(new(LoginRequest)),
		option.Response(http.StatusOK, new(LoginResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusForbidden, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)

	r.Post("/api/v1/users/token/refresh",
		option.OperationID("user-token-refresh"),
		option.Summary("Refresh an access token"),
		option.Description("Trades a valid refresh token for a fresh access/refresh pair. The account is reloaded rather than trusted from the token's claims. An access token is not accepted here, and the refresh token it returns replaces the one sent."),
		option.Tags("Users"),
		option.Request(new(RefreshTokenRequest)),
		option.Response(http.StatusOK, new(LoginResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
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
func ActivityDocs(r spec.Router) {
	r.Get("/api/v1/me/activity",
		option.OperationID("me-activity-list"),
		option.Summary("List my activities"),
		option.Description("Returns the authenticated user's activity history, newest first. The account is read from the access token, so a caller can only ever see their own activities. Paging is controlled with the limit and offset query parameters; limit defaults to 20 and is capped at 100."),
		option.Tags("Activities"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(ActivityListResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)
}
