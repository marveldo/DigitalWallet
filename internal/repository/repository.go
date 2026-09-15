package repository

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Repository struct {
	*UserRepository
	*ActivityRepository
	*TransactionRepository
	*gorm.DB
	trace.Tracer
	Logger *slog.Logger
}

type RepoConfig struct {
	*gorm.DB
	trace.Tracer
	Logger *slog.Logger
}

type RepoctxConfig struct {
	context.Context
	*gorm.DB
	trace.Span
	trace.Tracer
	Logger *slog.Logger
}

func NewRepository(cfg *RepoConfig) *Repository {
	user_repo := NewUserRepo(&UserRepositoryConfig{})
	activity_repo := NewActivityRepository(&ActivityRepositoryConfig{})
	transaction_repo := NewTransactionRepository(&TransactionRepositoryConfig{})
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Repository{
		DB:                    cfg.DB,
		Tracer:                cfg.Tracer,
		Logger:                logger,
		UserRepository:        &user_repo,
		ActivityRepository:    &activity_repo,
		TransactionRepository: &transaction_repo,
	}
}

type RepoCtx struct {
	context.Context
	*gorm.DB
	trace.Span
	trace.Tracer
	Logger *slog.Logger
}

func (r *Repository) NewRepoCtx(cfgs ...RepoctxConfig) *RepoCtx {
	context := context.Background()
	db := r.DB
	var span trace.Span
	var tracer trace.Tracer
	logger := r.Logger
	for _, cfg := range cfgs {
		if cfg.Logger != nil {
			logger = cfg.Logger
		}
		if cfg.Context != nil {
			context = cfg.Context
			span = trace.SpanFromContext(context)
		}
		if cfg.DB != nil {
			db = cfg.DB
		}
		if cfg.Tracer != nil {
			tracer = cfg.Tracer
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RepoCtx{
		Context: context,
		DB:      db,
		Span:    span,
		Tracer:  tracer,
		Logger:  logger,
	}
}



func (c *RepoCtx) Start(op string) (*RepoCtx, trace.Span) {
	if c.Tracer == nil {
		return c, trace.SpanFromContext(c.Context)
	}
	ctx, span := c.Tracer.Start(c.Context, "repository."+op)
	child := *c
	child.Context = ctx
	child.Span = span
	return &child, span
}
