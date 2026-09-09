package services

import (
	"context"
	"github/marveldo/eda-monolith/internal/repository"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Service struct {
	Repository *repository.Repository
	*gorm.DB
	trace.Tracer
}

type ServiceConfig struct {
	Repository *repository.Repository
}

type ServiceCtx struct {
	context.Context
	trace.Span
	trace.Tracer
}

type ServiceCtxConfig struct {
	context.Context
	trace.Span
	trace.Tracer
}

func NewService(cfg *ServiceConfig) *Service {
	return &Service{
		Repository: cfg.Repository,
		Tracer:     cfg.Repository.Tracer,
	}
}

func GetServiceCtx(cfgs ...ServiceCtxConfig) *ServiceCtx {
	context := context.Background()
	var span trace.Span
	var tracer trace.Tracer
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
	}
	return &ServiceCtx{
		Context: context,
		Span:    span,
		Tracer:  tracer,
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
