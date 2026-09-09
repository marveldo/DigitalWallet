package routes

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github/marveldo/eda-monolith/internal/api/services"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	"github.com/oaswrap/spec"
	"go.opentelemetry.io/otel/trace"
)

type Routes struct {
	*chi.Mux
	Logger    slog.Logger
	Services  *services.Service
	Validator *validator.Validate
	Spec      spec.Generator
	trace.Tracer
}

type SetupRoutesConfigParams struct {
	*chi.Mux
	Services *services.Service
	Spec     spec.Generator
	trace.Tracer
}

// StartChiRouter builds the base router with logging and CORS wired in.
// allowedOrigins comes from config.Config.AllowedOrigins (the ALLOWED_ORIGINS
// env var) — any http(s)://localhost[:port] or http(s)://127.0.0.1[:port]
// origin is always allowed on top of that list, regardless of port, so
// local/dev/testing frontends work without ALLOWED_ORIGINS needing to track
// whatever port they happen to be running on.
func StartChiRouter(allowedOrigins []string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(allowedOrigins))
	return r
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			if isLocalhostOrigin(origin) {
				return true
			}
			_, ok := allowed[origin]
			return ok
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// isLocalhostOrigin reports whether origin is http(s)://localhost,
// http(s)://127.0.0.1, or http(s)://[::1], on any port (or none). url.Parse
// strips the port via Hostname(), so this deliberately doesn't hand-parse
// the origin string itself.
func isLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func SetupRoutes(cfg SetupRoutesConfigParams) *Routes {
	routes := &Routes{
		Mux:       cfg.Mux,
		Logger:    *slog.Default(),
		Services:  cfg.Services,
		Spec:      cfg.Spec,
		Tracer:    cfg.Tracer,
		Validator: validator.New(),
	}

	r := cfg.Mux
	routes.SetupDocsRoutes()

	r.Get("/healthz", routes.HealthCheck)
	r.Route("/api", func(subR chi.Router) {
		subR.Route("/v1", func(v1 chi.Router) {
			v1.Post("/users", routes.CreateUser)
		})
	})
	return routes
}

// GetServiceCtx converts a request context into a service context, the same way
// Service.GetRepoCtx converts a service context into a repository one. It
// starts a span named for the operation, so a request is traced route ->
// service -> repository in one chain. The caller ends the span.
func (rt *Routes) GetServiceCtx(ctx context.Context, name string) (*services.ServiceCtx, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if rt.Tracer != nil {
		ctx, span = rt.Tracer.Start(ctx, name)
	}
	return services.GetServiceCtx(services.ServiceCtxConfig{
		Context: ctx,
		Span:    span,
		Tracer:  rt.Tracer,
	}), span
}
