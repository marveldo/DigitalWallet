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
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-playground/validator/v10"
	"github.com/oaswrap/spec"
	"go.opentelemetry.io/otel/trace"
)

type Routes struct {
	*chi.Mux
	Logger    *slog.Logger
	Services  *services.Service
	Validator *validator.Validate
	Spec      spec.Generator
	trace.Tracer
}

type SetupRoutesConfigParams struct {
	*chi.Mux
	Services *services.Service
	Spec     spec.Generator
	Logger   *slog.Logger
	trace.Tracer
}

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
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	routes := &Routes{
		Mux:       cfg.Mux,
		Logger:    logger,
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
			v1.Post("/users/otp/resend", routes.ResendOtp)
			v1.Post("/users/otp/verify", routes.VerifyOtp)
			v1.Post("/users/login", routes.Login)
			v1.Post("/users/token/refresh", routes.RefreshToken)
			v1.Post("/payments/webhook", routes.PaymentWebhook)

			v1.Group(func(m chi.Router) {
				m.Use(jwtauth.Verifier(routes.Services.JWTAuth))
				m.Use(routes.Authenticator)
				m.Use(routes.AuthMiddleWare)
				m.Patch("/users/{id}", routes.UpdateUser)
				m.Delete("/users/{id}", routes.DeleteUser)
				m.Get("/me/activity", routes.GetMyActivities)
				m.Post("/payments/deposits", routes.InitializeDeposit)
				m.Get("/me/transactions", routes.ListMyTransactions)
				m.Get("/transactions/{reference}", routes.GetTransaction)
			})
		})

	})
	return routes
}

func (rt *Routes) GetServiceCtx(ctx context.Context, name string) (*services.ServiceCtx, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if rt.Tracer != nil {
		ctx, span = rt.Tracer.Start(ctx, name)
	}
	return services.GetServiceCtx(services.ServiceCtxConfig{
		Context: ctx,
		Span:    span,
		Tracer:  rt.Tracer,
		Logger:  rt.Logger.With(slog.String("operation", name)),
	}), span
}
