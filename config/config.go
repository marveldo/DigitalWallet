package config



type Config struct {
	Database DatabaseConfig
	Uptrace
	BackgroundWorker AsynqBackgroundWorker
	AllowedOrigins []string

}

type DatabaseConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
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
}