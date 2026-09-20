package events

import "time"

const (
	EventUserCreated  = "user.created"
	EventUserLoggedIn = "user.loggedIn"

	EventPaymentInitialized     = "payment.initialized"
	EventPaymentWebhookReceived = "payment.webhook.received"

	EventTransferInitiated = "transfer.initiated"
)

type UserCreatedPayload struct {
	UserID    string
	Email     string
	FirstName string
	LastName  string
	CreatedAt time.Time
}

type UserLoggedInPayload struct {
	FirstName string
	LastName  string
	Email     string
	UserID    string
}

type PaymentInitializedPayload struct {
	UserID      string
	Reference   string
	Provider    string
	AmountMinor int64
	Currency    string
}

// TransferInitiatedPayload carries only the transfer id and enough context to
// log usefully. The worker reloads the transfer from Postgres rather than
// trusting amounts off the bus, so a stale or tampered payload cannot move a
// different amount than the one that was recorded.
type TransferInitiatedPayload struct {
	TransferID  string
	UserID      string
	AmountMinor int64
	Currency    string
}

type PaymentWebhookPayload struct {
	Reference   string
	Provider    string
	Event       string
	Status      string
	AmountMinor int64
	Currency    string
}
