package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
)

// DatabaseConfig configures the PostgreSQL connection pool.
type DatabaseConfig struct {
	Host               string        `env:"DB_HOST" envDefault:"localhost"`
	Port               string        `env:"DB_PORT" envDefault:"5432"`
	User               string        `env:"DB_USER" envDefault:"postgres"`
	Password           string        `env:"DB_PASSWORD" envDefault:"postgres"`
	Name               string        `env:"DB_NAME" envDefault:"postgres"`
	SSLMode            string        `env:"DB_SSLMODE" envDefault:"disable"`
	MaxConnection      int32         `env:"DB_MAX_CONNECTION" envDefault:"25"`
	MinConnection      int32         `env:"DB_MIN_CONNECTION" envDefault:"5"`
	MaxConnLifetime    time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"30m"`
	MaxConnIdleTime    time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"10m"`
	HealthCheckPeriod  time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"1m"`
	SlowQueryThreshold time.Duration `env:"DB_SLOW_QUERY_THRESHOLD" envDefault:"200ms"`
	QueryLogging       bool          `env:"DB_QUERY_LOGGING" envDefault:"true"`
}

// CORSConfig configures cross-origin request handling.
type CORSConfig struct {
	AllowOrigins     []string `env:"CORS_ALLOW_ORIGINS" envDefault:"*"`
	AllowMethods     []string `env:"CORS_ALLOW_METHODS" envDefault:"GET,POST,PUT,PATCH,DELETE,OPTIONS"`
	AllowHeaders     []string `env:"CORS_ALLOW_HEADERS" envDefault:"Content-Type,X-API-Key,Authorization"`
	AllowCredentials bool     `env:"CORS_ALLOW_CREDENTIALS" envDefault:"true"`
	MaxAge           int      `env:"CORS_MAX_AGE" envDefault:"86400"`
}

// LoggerConfig configures application logging
type LoggerConfig struct {
	MaxSize    int  `env:"LOG_MAX_SIZE" envDefault:"10"`
	MaxBackups int  `env:"LOG_MAX_BACKUPS" envDefault:"5"`
	MaxAge     int  `env:"LOG_MAX_AGE" envDefault:"7"`
	Compress   bool `env:"LOG_COMPRESS" envDefault:"true"`

	AppLevel    string `env:"LOG_APP_LEVEL" envDefault:"info"`
	AppFilename string `env:"LOG_APP_FILENAME" envDefault:"storage/logs/app.log"`

	HTTPLevel    string `env:"LOG_HTTP_LEVEL" envDefault:"info"`
	HTTPFilename string `env:"LOG_HTTP_FILENAME" envDefault:"storage/logs/http.log"`

	SQLLevel    string `env:"LOG_SQL_LEVEL" envDefault:"info"`
	SQLFilename string `env:"LOG_SQL_FILENAME" envDefault:"storage/logs/sql.log"`

	LimiterLevel    string `env:"LOG_LIMITER_LEVEL" envDefault:"info"`
	LimiterFilename string `env:"LOG_LIMITER_FILENAME" envDefault:"storage/logs/limiter.log"`
}

// LimiterConfig configures rate limiting.
type LimiterConfig struct {
	RequestsPerSecond int           `env:"LIMITER_RPS" envDefault:"10"`
	Burst             int           `env:"LIMITER_BURST" envDefault:"30"`
	NumShards         int           `env:"LIMITER_SHARDS" envDefault:"64"`
	CleanupInterval   time.Duration `env:"LIMITER_CLEANUP_INTERVAL" envDefault:"1m"`
	ClientTTL         time.Duration `env:"LIMITER_CLIENT_TTL" envDefault:"3m"`
	LogTTL            time.Duration `env:"LIMITER_LOG_TTL" envDefault:"30s"`
	IPWhitelist       []string      `env:"LIMITER_IP_WHITELIST" envDefault:"127.0.0.1,::1"`
}

// SecurityConfig configures request security limits and trusted proxies.
type SecurityConfig struct {
	MaxRequestBodySize int64    `env:"SECURITY_MAX_BODY_SIZE" envDefault:"2097152"`
	TrustedProxies     []string `env:"SECURITY_TRUSTED_PROXIES" envDefault:""`
}

// RedisConfig configures the Redis cache connection.
type RedisConfig struct {
	Enabled  bool   `env:"CACHE_ENABLED" envDefault:"false"`
	Host     string `env:"REDIS_HOST" envDefault:"localhost"`
	Port     string `env:"REDIS_PORT" envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

// StorageConfig configures file upload storage paths and URLs.
type StorageConfig struct {
	BaseDir string `env:"STORAGE_BASE_DIR" envDefault:"./storage/uploads"`
	BaseURL string `env:"STORAGE_BASE_URL" envDefault:"http://localhost:8080/uploads"`
}

// TelegramConfig configures Telegram notification settings.
type TelegramConfig struct {
	Token  string `env:"TELEGRAM_BOT_TOKEN" envDefault:""`
	ChatID string `env:"TELEGRAM_CHAT_ID" envDefault:""`
}

// RecaptchaConfig configures Google reCAPTCHA verification.
type RecaptchaConfig struct {
	SecretKey string  `env:"RECAPTCHA_SECRET_KEY" envDefault:""`
	MinScore  float64 `env:"RECAPTCHA_MIN_SCORE" envDefault:"0.5"`
	Mock      bool    `env:"RECAPTCHA_MOCK" envDefault:"true"`
}

// Config holds all configuration settings for the application.
type Config struct {
	ServerPort  string `env:"API_SERVER_PORT" envDefault:"8080"`
	Environment string `env:"API_ENVIRONMENT" envDefault:"development"`
	Version     string `env:"API_VERSION" envDefault:"v0.1.0"`
	Commit      string `env:"API_COMMIT" envDefault:"unknown"`
	Language    string `env:"API_LANGUAGE" envDefault:"en"`
	Timezone    string `env:"API_TIMEZONE" envDefault:"UTC"`

	// APIKey is the shared secret for authenticating protected routes.
	APIKey string `env:"API_KEY" envDefault:"dev-secret-api-key"`

	// Tracer debug dashboard authentication key and value.
	TracerDebugKey   string `env:"TRACER_DEBUG_KEY" envDefault:"t_debug"`
	TracerDebugValue string `env:"TRACER_DEBUG_VALUE" envDefault:"0604"`

	DB        DatabaseConfig
	CORS      CORSConfig
	Logger    LoggerConfig
	Limiter   LimiterConfig
	Security  SecurityConfig
	Redis     RedisConfig
	Storage   StorageConfig
	Telegram  TelegramConfig
	Recaptcha RecaptchaConfig
}

var (
	mu       sync.Mutex
	instance *Config
	loaded   bool
)

// NewConfig loads configuration from environment variables as a singleton.
func NewConfig() *Config {
	mu.Lock()
	defer mu.Unlock()

	if loaded {
		return instance
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		panic(fmt.Errorf("failed to load configuration: %w", err))
	}

	instance = cfg
	loaded = true
	return instance
}

// DSN returns the PostgreSQL connection string.
func (cfg *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s timezone=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name, cfg.DB.SSLMode, cfg.Timezone)
}
