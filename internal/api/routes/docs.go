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

func PaymentDocs(r spec.Router) {
	r.Post("/api/v1/payments/deposits",
		option.OperationID("payment-deposit-initialize"),
		option.Summary("Start a deposit"),
		option.Description("Opens a deposit against the caller's wallet and returns the provider checkout link to send the user to. The account is read from the access token, so a caller can only ever fund their own wallet. The amount is in major units (5000.00 means five thousand naira) and the currency selects which of the caller's wallets is credited, defaulting to NGN. Nothing is credited here: the deposit is recorded as PENDING and only becomes SUCCESS once the provider confirms it, so poll the transaction endpoint or wait for the wallet balance to change rather than treating this response as payment."),
		option.Tags("Payments"),
		option.Security(BearerSecurityScheme),
		option.Request(new(InitializeDepositRequest)),
		option.Response(http.StatusCreated, new(DepositResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
		option.Response(http.StatusBadGateway, new(ErrorResponse)),
		option.Response(http.StatusServiceUnavailable, new(ErrorResponse)),
	)

	r.Post("/api/v1/payments/webhook",
		option.OperationID("payment-webhook"),
		option.Summary("Receive a payment provider webhook"),
		option.Description("Called by the payment provider, not by clients. The raw body is authenticated against the provider's signature header and rejected if it does not match, which is why no bearer token is required. A verified webhook is queued for background settlement and acknowledged immediately, so a 200 means the notification was accepted rather than that the wallet has already been credited. The amount is never taken from this body: the provider is asked to confirm the transaction before anything is credited. Delivering the same event twice is safe and credits the wallet only once."),
		option.Tags("Payments"),
		option.Response(http.StatusOK, new(MessageResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusServiceUnavailable, new(ErrorResponse)),
	)

	r.Get("/api/v1/me/transactions",
		option.OperationID("me-transactions-list"),
		option.Summary("List my transactions"),
		option.Description("Returns the authenticated user's deposits, newest first. The account is read from the access token, so a caller can only ever see their own transactions. Paging is controlled with the limit and offset query parameters; limit defaults to 20 and is capped at 100."),
		option.Tags("Payments"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(TransactionListResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)

	r.Get("/api/v1/transactions/{reference}",
		option.OperationID("transaction-get"),
		option.Summary("Get a transaction"),
		option.Description("Returns one transaction by its reference, which is the value returned when the deposit was started. Poll this after a deposit to watch the status move from PENDING to SUCCESS or FAILED. A reference belonging to another account is reported as not found rather than forbidden, so the endpoint cannot be used to discover which references exist."),
		option.Tags("Payments"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(TransactionResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)
}
