package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestDSNPrefersURL(t *testing.T) {
	url := "postgresql://postgres.ref:pw@aws-0-eu-west-1.pooler.supabase.com:5432/postgres?sslmode=require"
	d := DatabaseConfig{URL: url, Host: "ignored", Port: 1}
	if got := d.DSN(); got != url {
		t.Fatalf("got %q, want the URL unchanged", got)
	}
}

func TestDSNFromFieldsDefaultsSSLModeToDisable(t *testing.T) {
	d := DatabaseConfig{Host: "localhost", Port: 5432, Username: "app", Password: "secret", Database: "eda"}
	want := "host=localhost user=app password=secret dbname=eda port=5432 sslmode=disable"
	if got := d.DSN(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDSNFromFieldsHonoursSSLMode(t *testing.T) {
	d := DatabaseConfig{Host: "h", Port: 5432, Username: "u", Password: "p", Database: "d", SSLMode: "require"}
	want := "host=h user=u password=p dbname=d port=5432 sslmode=require"
	if got := d.DSN(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDatabaseURLReadFromEnvironment(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATABASE_URL", "  postgresql://u:p@host:5432/db?sslmode=require  ")
	t.Setenv("JWT_SECRET_KEY", "test")

	v := viper.New()
	SetupViper(v)
	cfg := GetConfigFromViper(v)

	if got, want := cfg.Database.DSN(), "postgresql://u:p@host:5432/db?sslmode=require"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
