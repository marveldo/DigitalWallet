package routes

import (
	"log/slog"
	"net/http"

	openapiui "github.com/oaswrap/openapi-ui"
	uiconfig "github.com/oaswrap/openapi-ui/config"
	"github.com/oaswrap/spec"
	"github.com/oaswrap/spec/option"
)

const (
	DocsPath = "/docs"
	SpecPath = "/openapi.yaml"
)

// RouteDoc describes one domain's operations on the OpenAPI document.
type RouteDoc func(spec.Router)

// RoutesDocs is the registry GenerateDocs walks. Add a domain's doc function
// here when you add its handlers to SetupRoutes.
var RoutesDocs = []RouteDoc{
	HealthDocs,
	UserDocs,
}

// BearerSecurityScheme is the name the OpenAPI document gives the bearer
// token. GenerateDocs declares it once and each guarded operation references
// it by this name, which is what renders the padlock and the Authorize button
// in the UI.
const BearerSecurityScheme = "bearerAuth"

type GenerateDocsConfig struct {
	Title       string
	Version     string
	Description string
	ServerURL   string
	Routes      []RouteDoc
}

// GenerateDocs builds the OpenAPI document from the registry. The spec is
// described in code next to the chi tree, so there is no codegen step and no
// annotations to keep in sync.
func GenerateDocs(cfg *GenerateDocsConfig) spec.Generator {
	if cfg == nil {
		cfg = &GenerateDocsConfig{}
	}
	title := cfg.Title
	if title == "" {
		title = "eda-monolith"
	}
	version := cfg.Version
	if version == "" {
		version = "1.0.0"
	}

	opts := []option.OpenAPIOption{
		option.WithTitle(title),
		option.WithVersion(version),
		option.WithDescription(cfg.Description),
		// The UI is served by openapi-ui from SetupDocsRoutes, not by spec.
		option.WithDisableDocs(),
		option.WithSecurity(BearerSecurityScheme,
			option.SecurityHTTPBearer("bearer", "JWT"),
			option.SecurityDescription("Access token from /api/v1/users/login, sent as `Authorization: Bearer <token>`. Refresh tokens are not accepted."),
		),
	}
	if cfg.ServerURL != "" {
		opts = append(opts, option.WithServer(cfg.ServerURL))
	}

	gen := spec.NewGenerator(opts...)

	routeDocs := cfg.Routes
	if routeDocs == nil {
		routeDocs = RoutesDocs
	}
	for _, doc := range routeDocs {
		doc(gen)
	}
	return gen
}

// SetupDocsRoutes serves the generated document at SpecPath and the openapi-ui
// viewer at DocsPath.
func (rt *Routes) SetupDocsRoutes() {
	rt.Mux.Get(SpecPath, rt.ServeOpenAPISpec)
	rt.Mux.Handle(DocsPath, rt.OpenAPIUIHandler())
	rt.Mux.Handle(DocsPath+"/*", rt.OpenAPIUIHandler())
}

func (rt *Routes) OpenAPIUIHandler() http.Handler {
	return openapiui.NewHandler(openapiui.SwaggerUI(uiconfig.Swagger{
		Title:       rt.Spec.Config().Title,
		OpenAPIYAML: SpecPath,
		BasePath:    DocsPath + "/",
		ShowTopBar:  true,
	}))
}

func (rt *Routes) ServeOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	doc, err := rt.Spec.MarshalYAML()
	if err != nil {
		rt.Logger.Error("failed generating openapi document", slog.Any("error", err))
		rt.WriteError(w, rt.Logger, nil, http.StatusInternalServerError, "Could not generate OpenAPI document", err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(doc); err != nil {
		rt.Logger.Error("failed writing openapi document", slog.Any("error", err))
	}
}
