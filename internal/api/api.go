package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github/marveldo/eda-monolith/internal/api/routes"
	"github/marveldo/eda-monolith/internal/api/services"

	"go.opentelemetry.io/otel/trace"
)

type StartApiConfigParams struct {
	Services       *services.Service
	Logger         *slog.Logger
	Tracer         trace.Tracer
	Port           int
	AllowedOrigins []string
	DocsConfig     *routes.GenerateDocsConfig
}

// NewApiServer builds the router, the OpenAPI document and the *http.Server
// without listening, so callers control startup and shutdown themselves.
func NewApiServer(cfg StartApiConfigParams) *http.Server {
	mux := routes.StartChiRouter(cfg.AllowedOrigins)
	docs := routes.GenerateDocs(cfg.DocsConfig)

	rts := routes.SetupRoutes(routes.SetupRoutesConfigParams{
		Mux:      mux,
		Services: cfg.Services,
		Spec:     docs,
		Logger:   cfg.Logger,
		Tracer:   cfg.Tracer,
	})

	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: rts.Mux,
	}
}
