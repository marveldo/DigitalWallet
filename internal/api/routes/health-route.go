package routes

import (
	"net/http"

	"github.com/oaswrap/spec"
	"github.com/oaswrap/spec/option"
)

func HealthDocs(r spec.Router) {
	r.Get("/healthz",
		option.OperationID("health-check"),
		option.Summary("Health check"),
		option.Description("Reports whether the service is up."),
		option.Tags("System"),
		option.Response(http.StatusOK, new(HealthResponse)),
	)
}

func (rt *Routes) HealthCheck(w http.ResponseWriter, r *http.Request) {
	rt.WriteJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}
