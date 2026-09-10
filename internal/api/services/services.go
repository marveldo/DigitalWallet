package services

import (
	"context"
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/shared"

	"github/marveldo/eda-monolith/internal/api/events"
	"github/marveldo/eda-monolith/internal/otp"
	"github/marveldo/eda-monolith/internal/repository"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Service struct {
	Repository *repository.Repository
	*gorm.DB
	trace.Tracer
	Logger   *slog.Logger
	EventBus *events.EventBus
	*otp.Store
}

type ServiceConfig struct {
	Repository *repository.Repository
	Logger     *slog.Logger
	EventBus   *events.EventBus
	Store      *otp.Store
}

type ServiceCtx struct {
	context.Context
	trace.Span
	trace.Tracer
	Logger *slog.Logger
}

type ServiceCtxConfig struct {
	context.Context
	trace.Span
	trace.Tracer
	Logger *slog.Logger
}

func NewService(cfg *ServiceConfig) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = cfg.Repository.Logger
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		Repository: cfg.Repository,
		DB:         cfg.Repository.DB,
		Tracer:     cfg.Repository.Tracer,
		Logger:     logger,
		EventBus:   cfg.EventBus,
		Store:      cfg.Store,
	}
}

func GetServiceCtx(cfgs ...ServiceCtxConfig) *ServiceCtx {
	context := context.Background()
	var span trace.Span
	var tracer trace.Tracer
	var logger *slog.Logger
	for _, cfg := range cfgs {
		if cfg.Context != nil {
			context = cfg.Context
			span = trace.SpanFromContext(context)
		}
		if cfg.Span != nil {
			span = cfg.Span
		}
		if cfg.Tracer != nil {
			tracer = cfg.Tracer
		}
		if cfg.Logger != nil {
			logger = cfg.Logger
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ServiceCtx{
		Context: context,
		Span:    span,
		Tracer:  tracer,
		Logger:  logger,
	}
}

func (s *Service) GetRepoCtx(cfgs ...ServiceCtxConfig) *repository.RepoCtx {
	ctx := context.Background()
	var span trace.Span
	for _, cfg := range cfgs {
		if cfg.Context != nil {
			ctx = cfg.Context
			span = trace.SpanFromContext(ctx)
		}
		if cfg.Span != nil {
			span = cfg.Span
		}
	}
	return &repository.RepoCtx{
		Context: ctx,
		DB:      s.Repository.DB,
		Tracer:  s.Tracer,
		Span:    span,
	}
}

func (s *Service) PublishEvent(ctx *ServiceCtx, eventName string, payload any) {
	if s.EventBus == nil {
		return
	}
	s.EventBus.Publish(ctx.Context, eventName, payload)
}

func (c *ServiceCtx) Start(op string) (*ServiceCtx, trace.Span) {
	if c.Tracer == nil {
		return c, trace.SpanFromContext(c.Context)
	}
	ctx, span := c.Tracer.Start(c.Context, "service."+op)
	child := *c
	child.Context = ctx
	child.Span = span
	return &child, span
}

func (c *ServiceCtx) Fail(appErr *shared.AppError) *shared.AppError {
	if appErr == nil || c.Span == nil {
		return appErr
	}
	if appErr.Err != nil {
		c.Span.RecordError(appErr.Err)
	}
	if appErr.Code == 0 || appErr.Code >= http.StatusInternalServerError {
		msg := appErr.Message
		if msg == "" {
			msg = appErr.Error()
		}
		c.Span.SetStatus(codes.Error, msg)
	}
	return appErr
}
