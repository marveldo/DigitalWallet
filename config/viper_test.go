package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestGetViperInstance(t *testing.T) {
	if got := GetViperInstance(); got != viperInstance {
		t.Fatalf("GetViperInstance() = %p, want the package viper instance %p", got, viperInstance)
	}
}

func TestSetupViper(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".env", []byte("DB_HOST=file-host\nDB_PORT=5432\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_HOST", "environment-host")

	v := viper.New()
	SetupViper(v)

	if got := v.GetString("DB_HOST"); got != "environment-host" {
		t.Fatalf("DB_HOST = %q, want environment-host", got)
	}
	if got := v.GetInt("DB_PORT"); got != 5432 {
		t.Fatalf("DB_PORT = %d, want 5432", got)
	}
}

func TestGetConfigFromViper(t *testing.T) {
	v := viper.New()
	v.Set("DB_HOST", "localhost")
	v.Set("DB_PORT", 5432)
	v.Set("DB_USERNAME", "app")
	v.Set("DB_PASSWORD", "secret")
	v.Set("DB_DATABASE", "eda")
	v.Set("SKIP_AUTO_MIGRATE", true)
	v.Set("UPTRACE_DSN", "https://example.com/project")
	v.Set("UPTRACE_SERVICE_NAME", "eda-api")
	v.Set("UPTRACE_SERVICE_VERSION", "1.2.3")
	v.Set("ASYNC_REDIS_URL", "redis://localhost:6379/1")
	v.Set("ASYNC_REDIS_NAMESPACE", "eda")
	v.Set("ALLOWED_ORIGINS", []string{"http://localhost:3000", "https://example.com"})
	v.Set("JWT_SECRET_KEY", "test-signing-key")
	v.Set("JWT_ACCESS_TOKEN_EXPIRY_HOURS", 2)
	v.Set("JWT_REFRESH_TOKEN_EXPIRY_HOURS", 48)

	want := &Config{
		Database: DatabaseConfig{
			Host: "localhost", Port: 5432, Username: "app", Password: "secret",
			Database: "eda", SkipAutoMigrate: true,
		},
		Uptrace: Uptrace{
			UptraceDSN: "https://example.com/project", UptraceServiceName: "eda-api",
			UptraceServiceVersion: "1.2.3",
		},
		BackgroundWorker: AsynqBackgroundWorker{
			RedisUrl: "redis://localhost:6379/1", RedisNamespace: "eda",
		},
		AllowedOrigins: []string{"http://localhost:3000", "https://example.com"},
		JWT: JWTConfig{
			SecretKey: "test-signing-key", AccessTokenExpiry: 2, RefreshTokenExpiry: 48,
		},
	}

	if got := GetConfigFromViper(v); !reflect.DeepEqual(got, want) {
		t.Errorf("GetConfigFromViper() = %#v, want %#v", got, want)
	}
}

func TestGetConfigFromViperDefaults(t *testing.T) {
	v := viper.New()
	// JWT_SECRET_KEY has no default: GetConfigFromViper panics without it.
	v.Set("JWT_SECRET_KEY", "test-signing-key")

	got := GetConfigFromViper(v)
	want := &Config{
		BackgroundWorker: AsynqBackgroundWorker{
			RedisUrl: "redis://localhost:6379", RedisNamespace: "asynq",
		},
		AllowedOrigins: []string{"http://localhost:3000"},
		JWT:            JWTConfig{SecretKey: "test-signing-key"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetConfigFromViper() with empty config = %#v, want %#v", got, want)
	}
}

func TestGetAllowedOrigins(t *testing.T) {
	v := viper.New()
	v.Set("ALLOWED_ORIGINS", []string{"https://one.example", "https://two.example"})

	want := []string{"https://one.example", "https://two.example"}
	if got := getAllowedOrigins(v); !reflect.DeepEqual(got, want) {
		t.Errorf("getAllowedOrigins() = %#v, want %#v", got, want)
	}
}

func TestGetConfigFromViperPanicsWithoutJWTSecret(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("GetConfigFromViper() did not panic with JWT_SECRET_KEY unset")
		}
	}()
	GetConfigFromViper(viper.New())
}
