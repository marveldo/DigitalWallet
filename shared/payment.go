package shared

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrProviderUnavailable = errors.New("payment provider unavailable")
	ErrInvalidSignature    = errors.New("invalid webhook signature")
	ErrReferenceNotFound   = errors.New("payment reference not found at provider")
	ErrUnhandledWebhook    = errors.New("unhandled webhook event")
)

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "PENDING"
	PaymentSuccess   PaymentStatus = "SUCCESS"
	PaymentFailed    PaymentStatus = "FAILED"
	PaymentAbandoned PaymentStatus = "ABANDONED"
)

type InitializePaymentRequest struct {
	Reference   string
	Amount      Money
	Currency    string
	Email       string
	CallbackURL string
	Metadata    map[string]string
}

type InitializePaymentResponse struct {
	Reference        string
	AuthorizationURL string
	AccessCode       string
	Raw              json.RawMessage
}

type PaymentVerification struct {
	Reference   string
	Status      PaymentStatus
	Amount      Money
	Currency    string
	ProviderRef string
	Raw         json.RawMessage
}

type WebhookEvent struct {
	Event     string
	Reference string
	Status    PaymentStatus
	Amount    Money
	Currency  string
}

type PaymentProvider interface {
	Name() string
	Initialize(ctx context.Context, req InitializePaymentRequest) (*InitializePaymentResponse, error)
	Verify(ctx context.Context, reference string) (*PaymentVerification, error)
	VerifyWebhookSignature(signature string, body []byte) error
	ParseWebhook(body []byte) (*WebhookEvent, error)
	// Refund returns amount of a settled payment to the payer.
	Refund(ctx context.Context, reference string, amount Money) error
}
