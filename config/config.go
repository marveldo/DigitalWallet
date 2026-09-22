package config

import (
	"fmt"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Uptrace
	BackgroundWorker AsynqBackgroundWorker
	AllowedOrigins   []string
	Port             int
	ResendEmail      string
	ResendKey        string
	EmailProvider    string
	OTP              OTPConfig
	Payment          PaymentConfig
	JWT              JWTConfig
	TigerBeetle      TigerBeetleConfig
}

type TigerBeetleConfig struct {
	ClusterID uint64
	Addresses []string
}

type JWTConfig struct {
	SecretKey          string
	AccessTokenExpiry  uint64
	RefreshTokenExpiry uint64
}

type OTPConfig struct {
	Length      int
	TTL         time.Duration
	MaxAttempts int
}

type DatabaseConfig struct {
	// URL is a full connection string, such as the one Supabase hands out.
	// When it is set it is used as-is and the fields below are ignored.
	URL             string
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	SSLMode         string
	SkipAutoMigrate bool
}

// DSN returns what gorm should connect with: the URL when one is configured,
// otherwise a key/value DSN assembled from the individual fields.
func (d DatabaseConfig) DSN() string {
	if d.URL != "" {
		return d.URL
	}
	sslMode := d.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		d.Host, d.Username, d.Password, d.Database, d.Port, sslMode)
}

type Uptrace struct {
	UptraceDSN            string
	UptraceServiceName    string
	UptraceServiceVersion string
}

type AsynqBackgroundWorker struct {
	RedisUrl       string
	RedisNamespace string
	Concurrency    int
	Username       string
	Password       string
	DB             int
}

type PaymentConfig struct {
	Provider    string
	SecretKey   string
	BaseURL     string
	CallbackURL string
	// PollInterval is how often a pending deposit is verified with the
	// provider; after PollMaxRetries checks it is marked failed.
	PollInterval   time.Duration
	PollMaxRetries int
}
