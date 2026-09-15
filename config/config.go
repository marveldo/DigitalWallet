package config

import "time"

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
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	SkipAutoMigrate bool
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
