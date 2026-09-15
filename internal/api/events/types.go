package events

import "time"

const (
	EventUserCreated  = "user.created"
	EventUserLoggedIn = "user.loggedIn"

	EventPaymentInitialized     = "payment.initialized"
	EventPaymentWebhookReceived = "payment.webhook.received"
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

type PaymentWebhookPayload struct {
	Reference   string
	Provider    string
	Event       string
	Status      string
	AmountMinor int64
	Currency    string
}
