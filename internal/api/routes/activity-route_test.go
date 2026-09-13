package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github/marveldo/eda-monolith/internal/api/services"
	"github/marveldo/eda-monolith/internal/repository"

	"github.com/go-chi/chi/v5"
)

func newTestRoutes(t *testing.T) *Routes {
	t.Helper()
	repo := repository.NewRepository(&repository.RepoConfig{})
	svc := services.NewService(&services.ServiceConfig{Repository: repo, SecretKey: "test-secret"})
	return SetupRoutes(SetupRoutesConfigParams{
		Mux:      StartChiRouter(nil),
		Services: svc,
		Spec:     GenerateDocs(&GenerateDocsConfig{}),
	})
}

func TestActivityRouteRegistered(t *testing.T) {
	rt := newTestRoutes(t)
	var found bool
	chi.Walk(rt.Mux, func(method string, route string, h http.Handler, mw ...func(http.Handler) http.Handler) error {
		if method == "GET" && route == "/api/v1/me/activity" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("GET /api/v1/me/activity not registered")
	}
}

func TestActivityRouteRequiresAuth(t *testing.T) {
	rt := newTestRoutes(t)
	rec := httptest.NewRecorder()
	rt.Mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/me/activity", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without a token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestActivityInSpec(t *testing.T) {
	doc, err := GenerateDocs(&GenerateDocsConfig{}).MarshalYAML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "/api/v1/me/activity") {
		t.Fatal("spec missing /api/v1/me/activity")
	}
}

func TestPaymentRoutesInSpec(t *testing.T) {
	doc, err := GenerateDocs(&GenerateDocsConfig{}).MarshalYAML()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/v1/payments/deposits",
		"/api/v1/payments/webhook",
		"/api/v1/me/transactions",
		"/api/v1/transactions/{reference}",
	} {
		if !strings.Contains(string(doc), path) {
			t.Errorf("spec missing %s", path)
		}
	}
}
