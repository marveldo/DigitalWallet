package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github/marveldo/eda-monolith/shared"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	PaystackName            = "paystack"
	PaystackDefaultBaseURL  = "https://api.paystack.co"
	PaystackSignatureHeader = "x-paystack-signature"
)

type PaystackProvider struct {
	Client  *http.Client
	BaseURL string
	Secret  string
	trace.Tracer
	Logger *slog.Logger
}

type PaystackProviderConfig struct {
	SecretKey string
	BaseURL   string
	Client    *http.Client
	trace.Tracer
	Logger *slog.Logger
}

func NewPaystackProvider(cfg *PaystackProviderConfig) *PaystackProvider {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = PaystackDefaultBaseURL
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &PaystackProvider{
		Client:  client,
		BaseURL: baseURL,
		Secret:  cfg.SecretKey,
		Tracer:  cfg.Tracer,
		Logger:  logger.With(slog.String("payment_provider", PaystackName)),
	}
}

func (p *PaystackProvider) Name() string { return PaystackName }

func (p *PaystackProvider) StartSpanFromContext(ctx context.Context, op string) (context.Context, trace.Span) {
	if p.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return p.Tracer.Start(ctx, "payment.provider.paystack."+op)
}

type paystackEnvelope struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type paystackInitData struct {
	AuthorizationURL string `json:"authorization_url"`
	AccessCode       string `json:"access_code"`
	Reference        string `json:"reference"`
}

type paystackTransactionData struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	ID        int64  `json:"id"`
}

func (p *PaystackProvider) do(ctx context.Context, method, path string, body any) (*paystackEnvelope, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding paystack request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.BaseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("building paystack request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.Secret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", shared.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: reading paystack response: %v", shared.ErrProviderUnavailable, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, shared.ErrReferenceNotFound
	}

	var envelope paystackEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("%w: decoding paystack response (status %d): %v", shared.ErrProviderUnavailable, resp.StatusCode, err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: paystack returned %d: %s", shared.ErrProviderUnavailable, resp.StatusCode, envelope.Message)
	}
	if !envelope.Status {
		return nil, fmt.Errorf("paystack rejected %s %s: %s", method, path, envelope.Message)
	}
	return &envelope, nil
}

func (p *PaystackProvider) Initialize(ctx context.Context, req shared.InitializePaymentRequest) (*shared.InitializePaymentResponse, error) {
	ctx, span := p.StartSpanFromContext(ctx, "initialize")
	defer span.End()

	payload := map[string]any{
		"email":     req.Email,
		"amount":    req.Amount.Minor(),
		"reference": req.Reference,
	}
	if req.Currency != "" {
		payload["currency"] = req.Currency
	}
	if req.CallbackURL != "" {
		payload["callback_url"] = req.CallbackURL
	}
	if len(req.Metadata) > 0 {
		payload["metadata"] = req.Metadata
	}

	envelope, err := p.do(ctx, http.MethodPost, "/transaction/initialize", payload)
	if err != nil {
		p.Logger.ErrorContext(ctx, "could not initialize paystack transaction",
			slog.String("reference", req.Reference),
			slog.Any("error", err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	var data paystackInitData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("decoding paystack initialize data: %w", err)
	}

	reference := data.Reference
	if reference == "" {
		reference = req.Reference
	}

	p.Logger.InfoContext(ctx, "paystack transaction initialized",
		slog.String("reference", reference),
		slog.Int64("amount_minor", req.Amount.Minor()),
	)
	return &shared.InitializePaymentResponse{
		Reference:        reference,
		AuthorizationURL: data.AuthorizationURL,
		AccessCode:       data.AccessCode,
		Raw:              envelope.Data,
	}, nil
}

func (p *PaystackProvider) Verify(ctx context.Context, reference string) (*shared.PaymentVerification, error) {
	ctx, span := p.StartSpanFromContext(ctx, "verify")
	defer span.End()

	envelope, err := p.do(ctx, http.MethodGet, "/transaction/verify/"+reference, nil)
	if err != nil {
		p.Logger.ErrorContext(ctx, "could not verify paystack transaction",
			slog.String("reference", reference),
			slog.Any("error", err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	var data paystackTransactionData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("decoding paystack verify data: %w", err)
	}

	verification := &shared.PaymentVerification{
		Reference:   data.Reference,
		Status:      paystackStatus(data.Status),
		Amount:      shared.NewMoneyFromMinor(data.Amount),
		Currency:    data.Currency,
		ProviderRef: fmt.Sprintf("%d", data.ID),
		Raw:         envelope.Data,
	}
	if verification.Reference == "" {
		verification.Reference = reference
	}

	p.Logger.InfoContext(ctx, "paystack transaction verified",
		slog.String("reference", verification.Reference),
		slog.String("status", string(verification.Status)),
	)
	return verification, nil
}

func (p *PaystackProvider) VerifyWebhookSignature(signature string, body []byte) error {
	if p.Secret == "" {
		return fmt.Errorf("%w: no paystack secret configured", shared.ErrInvalidSignature)
	}
	if signature == "" {
		return fmt.Errorf("%w: missing %s header", shared.ErrInvalidSignature, PaystackSignatureHeader)
	}
	mac := hmac.New(sha512.New, []byte(p.Secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signature)))) {
		return shared.ErrInvalidSignature
	}
	return nil
}

type paystackWebhookBody struct {
	Event string                  `json:"event"`
	Data  paystackTransactionData `json:"data"`
}

func (p *PaystackProvider) ParseWebhook(body []byte) (*shared.WebhookEvent, error) {
	var parsed paystackWebhookBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding paystack webhook: %w", err)
	}
	if !strings.HasPrefix(parsed.Event, "charge.") && !strings.HasPrefix(parsed.Event, "transaction.") {
		return nil, fmt.Errorf("%w: %s", shared.ErrUnhandledWebhook, parsed.Event)
	}
	if parsed.Data.Reference == "" {
		return nil, fmt.Errorf("%w: webhook carried no reference", shared.ErrUnhandledWebhook)
	}
	return &shared.WebhookEvent{
		Event:     parsed.Event,
		Reference: parsed.Data.Reference,
		Status:    paystackStatus(parsed.Data.Status),
		Amount:    shared.NewMoneyFromMinor(parsed.Data.Amount),
		Currency:  parsed.Data.Currency,
	}, nil
}

func paystackStatus(status string) shared.PaymentStatus {
	switch strings.ToLower(status) {
	case "success":
		return shared.PaymentSuccess
	case "failed", "reversed":
		return shared.PaymentFailed
	case "abandoned":
		return shared.PaymentAbandoned
	default:
		return shared.PaymentPending
	}
}

var _ shared.PaymentProvider = (*PaystackProvider)(nil)
