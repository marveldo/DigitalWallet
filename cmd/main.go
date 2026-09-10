package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github/marveldo/eda-monolith/config"
	"github/marveldo/eda-monolith/internal/api"
	"github/marveldo/eda-monolith/internal/api/events"
	"github/marveldo/eda-monolith/internal/api/routes"
	"github/marveldo/eda-monolith/internal/api/services"
	"github/marveldo/eda-monolith/internal/queues"
	"github/marveldo/eda-monolith/internal/repository"
	dbmodels "github/marveldo/eda-monolith/internal/repository/db"
	"github/marveldo/eda-monolith/telemetry"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type StartServerConfig struct {
	fx.In
	*config.Config
	*viper.Viper
	Services *services.Service
	*slog.Logger
	trace.Tracer
}

type StartServicesConfig struct {
	fx.In
	*config.Config
	*repository.Repository
	trace.Tracer
	*slog.Logger
	EventBus *events.EventBus
}

func main() {
	fx_instance := fx.New(
		fx.Provide(
			NewViperInstance,
			StartConfigFromViper,
			StartContext,
			StartNewLogger,
			StartTracer,
			StartNewDB,
			StartNewRepo,
			StartNewEventBus,
			StartNewServices,
			StartNewQueueWorker,
		),
		fx.Invoke(func(lc fx.Lifecycle, cfg StartServerConfig) {

			server := api.NewApiServer(BuildApiConfig(&cfg))
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go StartServer(server, cfg.Logger)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					cfg.Logger.Info("shutting down api")
					return server.Shutdown(ctx)
				},
			})
		}),
		fx.Invoke(func(worker *queues.AsynqWorkerStruct) {}),
	)
	fx_instance.Run()
}

func BuildApiConfig(cfg *StartServerConfig) api.StartApiConfigParams {
	return api.StartApiConfigParams{
		Services:       cfg.Services,
		Logger:         cfg.Logger,
		Tracer:         cfg.Tracer,
		Port:           cfg.Viper.GetInt("PORT"),
		AllowedOrigins: cfg.Config.AllowedOrigins,
		DocsConfig: &routes.GenerateDocsConfig{
			Title:       "EDA Monolith",
			Version:     "1.0.0",
			Description: "HTTP API for the eda-monolith service.",
			ServerURL:   cfg.Viper.GetString("SERVER_URL"),
			Routes:      routes.RoutesDocs,
		},
	}
}

func StartServer(server *http.Server, logger *slog.Logger) {
	logger.Info("about to start api", slog.String("addr", server.Addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Errorf("api server failed: %w", err))
	}
}

func NewViperInstance(lc fx.Lifecycle) *viper.Viper {
	viperInstance := config.GetViperInstance()
	config.SetupViper(viperInstance)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				viperInstance.WatchConfig()
				fmt.Printf("stopped watching file")
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
	return viperInstance
}

func StartConfigFromViper(lc fx.Lifecycle, v *viper.Viper) *config.Config {
	return config.GetConfigFromViper(v)
}

func StartContext() context.Context {
	ctx := context.Background()
	return ctx
}

// StartNewLogger builds the app-wide structured logger and installs it as the
// slog default, so any package that reaches for slog.Default() (rather than
// taking the injected *slog.Logger) still writes in the same format.
func StartNewLogger(lc fx.Lifecycle, v *viper.Viper) *slog.Logger {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(v.GetString("LOG_LEVEL"))); err != nil {
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if v.GetString("LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With(slog.String("service", "eda-monolith"))
	slog.SetDefault(logger)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
	return logger
}

func StartTracer(lc fx.Lifecycle, ctx context.Context, cfg *config.Config) trace.Tracer {
	tracer := telemetry.StartAppTracer(ctx, telemetry.TracerConfig{
		AppName:           "EDA Monolith",
		AppServiceName:    cfg.UptraceServiceName,
		AppServiceVersion: cfg.UptraceServiceVersion,
		DSN:               cfg.UptraceDSN,
	})
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
	return tracer
}

func StartNewDB(lc fx.Lifecycle, app_cfg *config.Config, logger *slog.Logger) *gorm.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		app_cfg.Database.Host,
		app_cfg.Database.Username,
		app_cfg.Database.Password,
		app_cfg.Database.Database,
		app_cfg.Database.Port,
	)

	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{TranslateError: true})
	if err != nil {
		panic(err)
	}

	if app_cfg.Database.SkipAutoMigrate {
		logger.Warn("SKIP_AUTO_MIGRATE is set, skipping AutoMigrate — schema must already be up to date")
	} else {
		migrationModels := []any{
			&dbmodels.User{},
		}
		// AutoMigrate each model independently rather than one call for all
		// of them: GORM's AutoMigrate(models...) stops at the first error, so
		// a single failure would otherwise abort migration for every model
		// after it, not just the one that hit the error.
		for _, model := range migrationModels {
			if err := db.AutoMigrate(model); err != nil {
				panic(err)
			}
		}
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
	})
	return db
}

func StartNewRepo(lc fx.Lifecycle, db *gorm.DB, tracer trace.Tracer, logger *slog.Logger, app_cfg *config.Config) *repository.Repository {
	return repository.NewRepository(&repository.RepoConfig{
		DB:     db,
		Tracer: tracer,
		Logger: logger.With(slog.String("layer", "repository")),
	})
}

func StartNewServices(lc fx.Lifecycle, cfg StartServicesConfig) *services.Service {
	srvs := services.NewService(&services.ServiceConfig{
		Repository: cfg.Repository,
		Logger:     cfg.Logger.With(slog.String("layer", "services")),
		EventBus:   cfg.EventBus,
	})
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
	return srvs
}

func StartNewQueueWorker(lc fx.Lifecycle, app_cfg *config.Config, repo *repository.Repository, logger *slog.Logger) (*queues.AsynqWorkerStruct, error) {
	worker, err := queues.NewAsyncWorker(&queues.AsynqWorkerConfig{
		Config:     &app_cfg.BackgroundWorker,
		Repository: repo,
		Logger:     logger,
	})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := worker.Start(); err != nil {
				logger.Error("queue worker not started, background tasks will not run",
					slog.Any("error", err))
				return nil
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			worker.Shutdown()
			return nil
		},
	})
	return worker, nil
}

// StartNewEventBus is the in-process, synchronous pub-sub that services emit
// domain events to. RegisterListeners is what actually reacts to them — this
// constructor just builds the bus.
func StartNewEventBus(logger *slog.Logger) *events.EventBus {
	return events.NewEventBus(&events.EventBusConfig{Logger: logger})
}

// RegisterListeners wires each domain's subscribers onto the bus. This is the
// seam between the bus and the queue: the bus itself knows nothing about asynq,
// and the worker knows nothing about domain events.
func RegisterListeners(bus *events.EventBus, worker *queues.AsynqWorkerStruct, logger *slog.Logger) {
	events.RegisterEmailListeners(bus, worker)
	logger.Info("event listeners registered")
}
