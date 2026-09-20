package routes

import (
	"net/http"

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
		option.Response(http.StatusCreated, new(LoginResponse)),
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

func WalletDocs(r spec.Router) {
	r.Post("/api/v1/wallets",
		option.OperationID("wallet-create"),
		option.Summary("Create a wallet"),
		option.Description("Opens a wallet in the given currency for the authenticated user, defaulting to NGN. A user can hold one wallet per currency, so asking for a currency they already have returns 409. Every wallet is given a unique ten-digit account number, which is what other people use to send it money. The wallet starts with a zero balance; fund it by starting a deposit in the same currency."),
		option.Tags("Wallets"),
		option.Security(BearerSecurityScheme),
		option.Request(new(CreateWalletRequest)),
		option.Response(http.StatusCreated, new(WalletResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusConflict, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)

	r.Get("/api/v1/me/wallets",
		option.OperationID("me-wallets-list"),
		option.Summary("List my wallets"),
		option.Description("Returns the authenticated user's wallets with their current balances in major units, each with the ten-digit account number other people use to send it money. Balances are read from the ledger, so they include every settled deposit and transfer."),
		option.Tags("Wallets"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(WalletListResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusBadGateway, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)

	r.Get("/api/v1/wallets/{id}",
		option.OperationID("wallet-get"),
		option.Summary("Get a wallet"),
		option.Description("Returns one of the authenticated user's wallets with its current balance and its ten-digit account number. A wallet belonging to another account is reported as not found rather than forbidden."),
		option.Tags("Wallets"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(WalletResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusBadGateway, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)
}

func PaymentDocs(r spec.Router) {
	r.Post("/api/v1/payments/deposits",
		option.OperationID("payment-deposit-initialize"),
		option.Summary("Start a deposit"),
		option.Description("Opens a deposit against the caller's wallet and returns the provider checkout link to send the user to. The account is read from the access token, so a caller can only ever fund their own wallet. The wallet_id must be a wallet the caller owns; a wallet belonging to someone else is reported as not found, and a frozen or closed wallet is rejected with 422. The amount is in major units of that wallet's currency (5000.00 in an NGN wallet means five thousand naira). Nothing is credited here: the deposit is recorded as PENDING and only becomes SUCCESS once the provider confirms it, so poll the transaction endpoint or wait for the wallet balance to change rather than treating this response as payment."),
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
		option.Description("Returns the authenticated user's money history across every wallet they hold, newest first: deposits and wallet-to-wallet transfers in one list. Each row carries a kind of DEPOSIT or TRANSFER, and a direction of CREDIT or DEBIT relative to the caller, which is what tells money received from money sent on the same transfer. The account is read from the access token, so a caller can only ever see their own history. Settled rows are read from the ledger and are always SUCCESS; anything still PENDING or already FAILED is read from the database, because money that never moved leaves no ledger entry. Paging is controlled with the limit and offset query parameters; limit defaults to 20 and is capped at 100. The two sources are merged in memory up to a bounded window, so total counts that window rather than every row that has ever existed, and pages far beyond it are not served."),
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
		option.Description("Returns one transaction by its reference, which is the value returned when the deposit was started or the transfer was sent. Both kinds resolve here, so a reference taken from the transaction list always reads back. Poll this to watch the status move from PENDING to SUCCESS or FAILED. A reference belonging to another account is reported as not found rather than forbidden, so the endpoint cannot be used to discover which references exist."),
		option.Tags("Payments"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(TransactionResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)
}

func TransferDocs(r spec.Router) {
	r.Post("/api/v1/transfers",
		option.OperationID("transfer-initiate"),
		option.Summary("Send money to another account"),
		option.Description("Moves money from one of the caller's wallets to the wallet behind account_number. The sending wallet is read against the access token, so a caller can only ever spend their own money; a wallet belonging to someone else is reported as not found. The amount is in major units of the wallet's currency. Answers 202, not 201: the transfer is recorded and queued here, and the money moves in a background task, so the response is always PENDING and the outcome is read from GET /api/v1/transactions/{reference} or GET /api/v1/transfers/{id}. An unknown account number is reported as not found in the same terms whatever the reason, so the endpoint cannot be used to discover which account numbers exist. Both wallets must hold the same currency: a transfer between different currencies is rejected with 400 rather than converted. A balance that is clearly too low is rejected with 422 here, but that check is advisory, and a transfer accepted at this point can still end as FAILED with an insufficient funds reason once the ledger runs it. Sending twice creates two transfers, so treat a retry from the client as a new payment rather than a repeat of the first."),
		option.Tags("Transfers"),
		option.Security(BearerSecurityScheme),
		option.Request(new(TransferRequest)),
		option.Response(http.StatusAccepted, new(TransferResponse)),
		option.Response(http.StatusBadRequest, new(ErrorResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusUnprocessableEntity, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)

	r.Get("/api/v1/transfers/{id}",
		option.OperationID("transfer-get"),
		option.Summary("Get a transfer"),
		option.Description("Returns one transfer by the id handed back when it was started. Poll this to watch the status move from PENDING to SUCCESS or FAILED; a failed transfer carries a failure_reason explaining why, and no money moved. Either party to the transfer can read it, and a transfer the caller neither sent nor received is reported as not found rather than forbidden."),
		option.Tags("Transfers"),
		option.Security(BearerSecurityScheme),
		option.Response(http.StatusOK, new(TransferResponse)),
		option.Response(http.StatusUnauthorized, new(ErrorResponse)),
		option.Response(http.StatusNotFound, new(ErrorResponse)),
		option.Response(http.StatusInternalServerError, new(ErrorResponse)),
	)
}
