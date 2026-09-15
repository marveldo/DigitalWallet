package config

import (
	"github.com/spf13/viper"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var viperInstance = viper.New()

func GetViperInstance() *viper.Viper {
	return viperInstance
}

func SetupViper(v *viper.Viper) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting Path Directory , %v", err)
	}
	env_file_path := path.Join(dir, ".env")
	_, err = os.Stat(env_file_path)
	if os.IsNotExist(err) {
		log.Fatalf("Error getting Path Directory , %v", err)
	}
	v.SetConfigFile(env_file_path)
	if err = v.ReadInConfig(); err != nil {
		_, ok := err.(viper.ConfigFileNotFoundError)
		if ok {
			log.Fatalf("Config File Doesnt Exist in Specific Path, %v", err)
		} else {
			log.Fatalf("Error reading config file, %v", err)
		}
	}
	v.AutomaticEnv()
}

func GetConfigFromViper(v *viper.Viper) *Config {
	port := func() int {
		if v.IsSet("APP_PORT") {
			if v.GetInt("APP_PORT") < 1 {
				return 7001
			} else {
				return viper.GetInt("APP_PORT")
			}
		} else {
			return 7001
		}
	}()
	databaseConfig := DatabaseConfig{
		Host:     v.GetString("DB_HOST"),
		Port:     v.GetInt("DB_PORT"),
		Username: v.GetString("DB_USERNAME"),
		Password: v.GetString("DB_PASSWORD"),
		Database: v.GetString("DB_DATABASE"),
		SkipAutoMigrate: func() bool {
			if v.IsSet("SKIP_AUTO_MIGRATE") {
				return v.GetBool("SKIP_AUTO_MIGRATE")
			} else {
				return false
			}
		}(),
	}
	uptraceConfig := Uptrace{
		UptraceDSN:            v.GetString("UPTRACE_DSN"),
		UptraceServiceName:    v.GetString("UPTRACE_SERVICE_NAME"),
		UptraceServiceVersion: v.GetString("UPTRACE_SERVICE_VERSION"),
	}
	backgroundWorkerConfig := AsynqBackgroundWorker{
		RedisUrl: func() string {
			if v.IsSet("ASYNC_REDIS_URL") {
				ul := v.GetString("ASYNC_REDIS_URL")
				u, err := url.Parse(ul)
				if err != nil {
					log.Fatalf("Error parsing ASYNC_REDIS_URL: %v", err)
				}
				if u.Host == "" {
					log.Fatalf("Error parsing ASYNC_REDIS_URL: missing host")
				}
				return ul
			} else {
				return "redis://localhost:6379"
			}
		}(),
		RedisNamespace: func() string {
			if v.IsSet("ASYNC_REDIS_NAMESPACE") {
				return v.GetString("ASYNC_REDIS_NAMESPACE")
			} else {
				return "asynq"
			}
		}(),
		Concurrency: func() int {
			if v.IsSet("ASYNC_REDIS_CONCURRENCY") {
				con := v.GetInt("ASYNC_REDIS_CONCURRENCY")
				if con < 1 {
					return 10
				}
				return con
			} else {
				return 10
			}
		}(),
		DB: func() int {
			if v.IsSet("ASYNC_REDIS_DB") {
				con := v.GetInt("ASYNC_REDIS_DB")
				if con < 1 {
					return 10
				}
				return con
			} else {
				return 10
			}
		}(),
		Username: v.GetString("ASYNC_REDIS_USERNAME"),
		Password: v.GetString("ASYNC_REDIS_PASSWORD"),
	}

	otpConfig := OTPConfig{
		Length: func() int {
			if v.IsSet("OTP_LENGTH") && v.GetInt("OTP_LENGTH") > 0 {
				return v.GetInt("OTP_LENGTH")
			}
			return 6
		}(),
		TTL: func() time.Duration {
			if v.IsSet("OTP_TTL_MINUTES") && v.GetInt("OTP_TTL_MINUTES") > 0 {
				return time.Duration(v.GetInt("OTP_TTL_MINUTES")) * time.Minute
			}
			return 10 * time.Minute
		}(),
		MaxAttempts: func() int {
			if v.IsSet("OTP_MAX_ATTEMPTS") && v.GetInt("OTP_MAX_ATTEMPTS") > 0 {
				return v.GetInt("OTP_MAX_ATTEMPTS")
			}
			return 5
		}(),
	}

	tigerBeetleConfig := TigerBeetleConfig{
		ClusterID: v.GetUint64("TB_CLUSTER_ID"),
		Addresses: func() []string {
			addresses := v.GetStringSlice("TB_ADDRESSES")
			if len(addresses) == 0 {
				return []string{"3000"}
			}
			return addresses
		}(),
	}

	jwtConfig := JWTConfig{
		SecretKey: func() string {
			key := v.GetString("JWT_SECRET_KEY")
			if key == "" {
				panic("JWT_SECRET_KEY is not set: refusing to sign tokens with an empty key, anyone could forge one")
			}
			return key
		}(),
		AccessTokenExpiry:  uint64(v.GetInt("JWT_ACCESS_TOKEN_EXPIRY_HOURS")),
		RefreshTokenExpiry: uint64(v.GetInt("JWT_REFRESH_TOKEN_EXPIRY_HOURS")),
	}

	paymentConfig := PaymentConfig{
		Provider: func() string {
			if v.IsSet("PAYMENT_PROVIDER") {
				return strings.ToLower(v.GetString("PAYMENT_PROVIDER"))
			}
			return "paystack"
		}(),
		SecretKey:   v.GetString("PAYSTACK_SECRET_KEY"),
		BaseURL:     v.GetString("PAYSTACK_BASE_URL"),
		CallbackURL: v.GetString("PAYMENT_CALLBACK_URL"),
		PollInterval: func() time.Duration {
			if v.IsSet("PAYMENT_POLL_INTERVAL_SECONDS") && v.GetInt("PAYMENT_POLL_INTERVAL_SECONDS") > 0 {
				return time.Duration(v.GetInt("PAYMENT_POLL_INTERVAL_SECONDS")) * time.Second
			}
			return time.Minute
		}(),
		PollMaxRetries: func() int {
			if v.IsSet("PAYMENT_POLL_MAX_RETRIES") && v.GetInt("PAYMENT_POLL_MAX_RETRIES") > 0 {
				return v.GetInt("PAYMENT_POLL_MAX_RETRIES")
			}
			return 30
		}(),
	}

	return &Config{
		Database:         databaseConfig,
		Uptrace:          uptraceConfig,
		BackgroundWorker: backgroundWorkerConfig,
		AllowedOrigins:   getAllowedOrigins(v),
		Port:             port,
		ResendEmail:      v.GetString("RESEND_API_EMAIL"),
		ResendKey:        v.GetString("RESEND_API_KEY"),
		EmailProvider:    v.GetString("EMAIL_PROVIDER"),
		OTP:              otpConfig,
		Payment:          paymentConfig,
		JWT:              jwtConfig,
		TigerBeetle:      tigerBeetleConfig,
	}
}

func getAllowedOrigins(v *viper.Viper) []string {
	allowedOrigins := v.GetStringSlice("ALLOWED_ORIGINS")
	if len(allowedOrigins) == 0 {
		log.Println("ALLOWED_ORIGINS is not set or empty, using default value: http://localhost:3000")
		return []string{"http://localhost:3000"}
	}
	return allowedOrigins
}
