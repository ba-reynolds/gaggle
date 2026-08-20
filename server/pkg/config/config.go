package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type AllConfigs struct {
	DBConfig      DBConfig
	ServerConfig  ServerConfig
	LoggingConfig LoggingConfig
	AppConfig     AppConfig
	AuthConfig    AuthConfig
	RedisConfig   RedisConfig
	MetricsConfig MetricsConfig
}

type DBConfig struct {
	DBAddress      string
	DBUser         string
	DBPassword     string
	DBName         string
	DBMaxOpenConns int
	DBMaxIdleConns int
	DBMaxIdleTime  time.Duration
}

type LoggingConfig struct {
	Filename string
}

type ServerConfig struct {
	ServerAddr         string
	ServerWriteTimeout time.Duration
	ServerReadTimeout  time.Duration
	ServerIdleTimeout  time.Duration
	ServerReqTimeout   time.Duration
}

type AppConfig struct {
	MediaDir               string
	SwaggerURL             string
	DefaultPaginationLimit int
	MaxPaginationLimit     int
}

type AuthConfig struct {
	JWTSecret                     string
	JWTIssuer                     string
	JWTAccessTokenExpirationTime  time.Duration
	JWTRefreshTokenExpirationTime time.Duration
	// CookieSecure marks the refresh-token cookie as Secure. Must be false for
	// plain-HTTP local development (browsers refuse to store Secure cookies
	// over http://localhost), and true behind TLS in production.
	CookieSecure bool
}

type RedisConfig struct {
	Address              string
	Password             string
	DB                   int
	FeedCacheTTL         time.Duration
	RateLimitMaxRequests int
	RateLimitWindow      time.Duration
}

// MetricsConfig drives the background host-stats sampler and metric retention.
type MetricsConfig struct {
	// HostSampleInterval is how often the sampler records a host snapshot.
	HostSampleInterval time.Duration
	// RetentionDays is how long page_views and host_metrics_samples rows are
	// kept before the hourly prune deletes them.
	RetentionDays int
}

func LoadConfig() (AllConfigs, error) {
	// Load .env file if available (but don't fail if missing)
	err := godotenv.Load()
	if err != nil {
		zap.L().Warn("failed to load .env file", zap.Error(err))
	}

	dbConfig := DBConfig{
		DBAddress:      getEnv("DB_ADDRESS", "127.0.0.1"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "password"),
		DBName:         getEnv("DB_NAME", "social"),
		DBMaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 30),
		DBMaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 15),
		DBMaxIdleTime:  time.Duration(getEnvInt("DB_MAX_IDLE_TIME", 60)) * time.Second,
	}

	loggingConfig := LoggingConfig{
		Filename: getEnv("LOGGING_FILENAME", "logs.log"),
	}

	serverConfig := ServerConfig{
		ServerAddr:         getEnv("SERVER_ADDR", "localhost:8080"),
		ServerWriteTimeout: time.Duration(getEnvInt("SERVER_WRITE_TIMEOUT", 30)) * time.Second,
		ServerReadTimeout:  time.Duration(getEnvInt("SERVER_READ_TIMEOUT", 10)) * time.Second,
		ServerIdleTimeout:  time.Duration(getEnvInt("SERVER_IDLE_TIMEOUT", 60)) * time.Second,
		ServerReqTimeout:   time.Duration(getEnvInt("SERVER_REQUEST_TIMEOUT", 60)) * time.Second,
	}

	appConfig := AppConfig{
		MediaDir:               getEnv("MEDIA_DIR", "media"),
		SwaggerURL:             fmt.Sprintf("%s/swagger/doc.json", serverConfig.ServerAddr),
		DefaultPaginationLimit: getEnvInt("PAGINATION_DEFAULT_LIMIT", 20),
		MaxPaginationLimit:     getEnvInt("PAGINATION_MAX_LIMIT", 100),
	}

	authConfig := AuthConfig{
		JWTSecret:                     getEnv("JWT_SECRET", "secret"),
		JWTIssuer:                     getEnv("JWT_ISSUER", "social"),
		JWTAccessTokenExpirationTime:  time.Duration(getEnvInt("JWT_ACCESS_TOKEN_EXPIRATION_TIME", 180)) * time.Second,
		JWTRefreshTokenExpirationTime: time.Duration(getEnvInt("JWT_REFRESH_TOKEN_EXPIRATION_TIME", 86400)) * time.Second,
		CookieSecure:                  getEnv("COOKIE_SECURE", "false") == "true",
	}

	redisConfig := RedisConfig{
		Address:              getEnv("REDIS_ADDRESS", "localhost:6379"),
		Password:             getEnv("REDIS_PASSWORD", ""),
		DB:                   getEnvInt("REDIS_DB", 0),
		FeedCacheTTL:         time.Duration(getEnvInt("REDIS_FEED_CACHE_TTL_SECONDS", 60)) * time.Second,
		RateLimitMaxRequests: getEnvInt("RATE_LIMIT_MAX_REQUESTS", 30),
		RateLimitWindow:      time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	}

	metricsConfig := MetricsConfig{
		HostSampleInterval: time.Duration(getEnvInt("METRICS_HOST_SAMPLE_SECONDS", 60)) * time.Second,
		RetentionDays:      getEnvInt("METRICS_RETENTION_DAYS", 90),
	}

	// Production fails fast: never boot with a shipped dev secret.
	if getEnv("APP_ENV", "development") == "production" {
		if !isStrongSecret(authConfig.JWTSecret) {
			return AllConfigs{}, fmt.Errorf("APP_ENV=production requires a real JWT_SECRET (got %q)", authConfig.JWTSecret)
		}
		if !isStrongSecret(dbConfig.DBPassword) {
			return AllConfigs{}, fmt.Errorf("APP_ENV=production requires a real DB_PASSWORD (got %q)", dbConfig.DBPassword)
		}
	}

	return AllConfigs{
		DBConfig:      dbConfig,
		LoggingConfig: loggingConfig,
		ServerConfig:  serverConfig,
		AppConfig:     appConfig,
		AuthConfig:    authConfig,
		RedisConfig:   redisConfig,
		MetricsConfig: metricsConfig,
	}, nil

}

// isStrongSecret rejects empty values and every secret default currently
// shipped anywhere in the repo (config.go, .env.example, compose.yaml).
func isStrongSecret(value string) bool {
	if value == "" {
		return false
	}
	for _, dev := range []string{"secret", "dev-secret-change-me", "change-me", "teeth", "password"} {
		if value == dev {
			return false
		}
	}
	return true
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	valueAsInt, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return valueAsInt
}
