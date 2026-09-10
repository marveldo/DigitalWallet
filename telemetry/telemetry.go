package telemetry

import (
	"context"

	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type TracerConfig struct {
	AppName           string
	AppServiceName    string
	AppServiceVersion string
	DSN               string
}


func StartAppTracer(ctx context.Context, cfg TracerConfig) (trace.Tracer, func(context.Context) error) {
	name := cfg.AppServiceName
	if name == "" {
		name = cfg.AppName
	}
	if name == "" {
		name = "eda-monolith"
	}

	shutdown := func(context.Context) error { return nil }
	if cfg.DSN != "" {
		uptrace.ConfigureOpentelemetry(
			uptrace.WithDSN(cfg.DSN),
			uptrace.WithServiceName(name),
			uptrace.WithServiceVersion(cfg.AppServiceVersion),
		)
		shutdown = uptrace.Shutdown
	}

	opts := []trace.TracerOption{}
	if cfg.AppServiceVersion != "" {
		opts = append(opts, trace.WithInstrumentationVersion(cfg.AppServiceVersion))
	}
	return otel.Tracer(name, opts...), shutdown
}
