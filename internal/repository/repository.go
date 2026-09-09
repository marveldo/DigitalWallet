package repository

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Repository struct {
	*UserRepository
	*gorm.DB
	trace.Tracer
}

type RepoConfig struct {
	*gorm.DB
	trace.Tracer
}

type RepoctxConfig struct {
	context.Context
	*gorm.DB
	trace.Span
	trace.Tracer
}


func NewRepository(cfg *RepoConfig) *Repository {
	user_repo := NewUserRepo(&UserRepositoryConfig{})
	return &Repository{
		DB: cfg.DB,
		Tracer: cfg.Tracer,
		UserRepository: &user_repo,
	}
}

type RepoCtx struct {
	context.Context
	*gorm.DB
	trace.Span
	trace.Tracer
}

func (r *Repository) NewRepoCtx(cfgs ...RepoctxConfig) *RepoCtx {
	context := context.Background()
	var db *gorm.DB
	var span trace.Span
	var tracer trace.Tracer
	for _, cfg := range cfgs {
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
	return &RepoCtx{
		Context: context,
		DB:      db,
		Span:    span,
		Tracer:  tracer,
	}
}
