package providers

import (
	"context"
	"fmt"
	"github/marveldo/eda-monolith/internal/queues"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"log/slog"

	"github.com/resend/resend-go/v3"
)

type ResendProvider struct {
	Client *resend.Client
	trace.Tracer
	Logger      *slog.Logger
	SenderEmail string
}

type ResendProviderConfig struct {
	SenderEmail string
	trace.Tracer
	ResendKey string
	Logger    *slog.Logger
}

func NewResendProvider(cfg *ResendProviderConfig) *ResendProvider {
	client := resend.NewClient(cfg.ResendKey)
	return &ResendProvider{
		Client:      client,
		Logger:      cfg.Logger,
		Tracer:      cfg.Tracer,
		SenderEmail: cfg.SenderEmail,
	}
}

func (p *ResendProvider) StartSpanFromContext(ctx context.Context) (context.Context, trace.Span) {
	if p.Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return p.Tracer.Start(ctx, "email.provider.resend")
}

func (p *ResendProvider) Send(ctx context.Context, payload queues.EmailPayload, body string) error {
	ctx, span := p.StartSpanFromContext(ctx)
	defer span.End()

	params := &resend.SendEmailRequest{
		From:    p.SenderEmail,
		To:      payload.To,
		Subject: payload.Subject,
		Html:    body,
	}

	sent, err := p.Client.Emails.SendWithContext(ctx, params)
	if err != nil {
		p.Logger.ErrorContext(ctx, "resend rejected the email",
			slog.Any("to", payload.To),
			slog.String("subject", payload.Subject),
			slog.Any("error", err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("sending email via resend: %w", err)
	}

	p.Logger.InfoContext(ctx, "email accepted by resend",
		slog.String("email_id", sent.Id),
		slog.Any("to", payload.To),
	)
	return nil
}
