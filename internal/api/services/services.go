package services

import (
	"context"
	"log/slog"

	"github/marveldo/eda-monolith/internal/api/events"
	"github/marveldo/eda-monolith/internal/repository"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Service struct {
	Repository *repository.Repository
	*gorm.DB
	trace.Tracer
	Logger   *slog.Logger
	EventBus *events.EventBus
}

type ServiceConfig struct {
	Repository *repository.Repository
	Logger     *slog.Logger
	EventBus   *events.EventBus
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
