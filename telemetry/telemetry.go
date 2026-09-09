package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type TracerConfig struct {
	AppName           string
	AppServiceName    string
	AppServiceVersion string
	DSN               string
}

// StartAppTracer returns the tracer shared by the route, service and
// repository layers.
//
// It reads from the global OpenTelemetry provider, and nothing installs one
// yet — so spans are created and discarded until an exporter is configured
// here. DSN is carried for that purpose (UPTRACE_DSN) but is not wired up.
func StartAppTracer(ctx context.Context, cfg TracerConfig) trace.Tracer {
	name := cfg.AppServiceName
	if name == "" {
		name = cfg.AppName
	}
	if name == "" {
		name = "eda-monolith"
	}

	opts := []trace.TracerOption{}
	if cfg.AppServiceVersion != "" {
		opts = append(opts, trace.WithInstrumentationVersion(cfg.AppServiceVersion))
	}
	return otel.Tracer(name, opts...)
}
