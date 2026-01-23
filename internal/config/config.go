package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Queue      QueueConfig      `mapstructure:"queue"`
	Providers  ProvidersConfig  `mapstructure:"providers"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Security   SecurityConfig   `mapstructure:"security"`
}

// AppConfig contains application-level configuration
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
	LogLevel    string `mapstructure:"log_level"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
}

// DatabaseConfig contains Elasticsearch configuration
type DatabaseConfig struct {
	URLs        []string `mapstructure:"urls"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	MaxRetries  int      `mapstructure:"max_retries"`
	Timeout     int      `mapstructure:"timeout"`
	IndexPrefix string   `mapstructure:"index_prefix"`
}

// RedisConfig contains Redis configuration
type RedisConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Password    string `mapstructure:"password"`
	DB          int    `mapstructure:"db"`
	PoolSize    int    `mapstructure:"pool_size"`
	MaxRetries  int    `mapstructure:"max_retries"`
	MinIdleConn int    `mapstructure:"min_idle_conn"`
}

// QueueConfig contains queue configuration
type QueueConfig struct {
	Type          string `mapstructure:"type"` // redis or kafka
	Workers       int    `mapstructure:"workers"`
	Concurrency   int    `mapstructure:"concurrency"`
	MaxRetries    int    `mapstructure:"max_retries"`
	RetryDelay    int    `mapstructure:"retry_delay"`
	MaxRetryDelay int    `mapstructure:"max_retry_delay"`
}

// ProvidersConfig contains external provider configurations
type ProvidersConfig struct {
	FCM      FCMConfig      `mapstructure:"fcm"`
	APNS     APNSConfig     `mapstructure:"apns"`
	SendGrid SendGridConfig `mapstructure:"sendgrid"`
	Twilio   TwilioConfig   `mapstructure:"twilio"`
	SMTP     SMTPConfig     `mapstructure:"smtp"`
	Webhook  WebhookConfig  `mapstructure:"webhook"`
}

// FCMConfig contains Firebase Cloud Messaging configuration
type FCMConfig struct {
	ServerKey       string `mapstructure:"server_key"`
	ProjectID       string `mapstructure:"project_id"`
	CredentialsPath string `mapstructure:"credentials_path"`
	Timeout         int    `mapstructure:"timeout"`
	MaxRetries      int    `mapstructure:"max_retries"`
}

// APNSConfig contains Apple Push Notification Service configuration
type APNSConfig struct {
	KeyID      string `mapstructure:"key_id"`
	TeamID     string `mapstructure:"team_id"`
	BundleID   string `mapstructure:"bundle_id"`
	KeyPath    string `mapstructure:"key_path"`
	Production bool   `mapstructure:"production"`
}

// SendGridConfig contains SendGrid configuration
type SendGridConfig struct {
	APIKey     string `mapstructure:"api_key"`
	FromEmail  string `mapstructure:"from_email"`
	FromName   string `mapstructure:"from_name"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// TwilioConfig contains Twilio configuration
type TwilioConfig struct {
	AccountSID string `mapstructure:"account_sid"`
	AuthToken  string `mapstructure:"auth_token"`
	FromNumber string `mapstructure:"from_number"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// SMTPConfig contains SMTP configuration
type SMTPConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	FromEmail  string `mapstructure:"from_email"`
	FromName   string `mapstructure:"from_name"`
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// WebhookConfig contains Webhook configuration
type WebhookConfig struct {
	Timeout    int    `mapstructure:"timeout"`
	MaxRetries int    `mapstructure:"max_retries"`
	Secret     string `mapstructure:"secret"` // For signing payloads
}

// MonitoringConfig contains monitoring configuration
type MonitoringConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Port      int    `mapstructure:"port"`
	Path      string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"`
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	JWTSecret       string     `mapstructure:"jwt_secret"`
	APIKeyHeader    string     `mapstructure:"api_key_header"`
	RateLimit       int        `mapstructure:"rate_limit"`
	RateLimitWindow int        `mapstructure:"rate_limit_window"`
	CORS            CORSConfig `mapstructure:"cors"`
}

// CORSConfig contains CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// Load loads configuration from various sources
func Load() (*Config, error) {
	// Set default values
	viper.SetDefault("app.name", "FreeRangeNotify")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.debug", true)
	viper.SetDefault("app.log_level", "info")

	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.idle_timeout", 120)

	viper.SetDefault("database.urls", []string{"http://localhost:9200"})
	viper.SetDefault("database.max_retries", 3)
	viper.SetDefault("database.timeout", 30)
	viper.SetDefault("database.index_prefix", "freerange")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.min_idle_conn", 10)

	viper.SetDefault("queue.type", "redis")
	viper.SetDefault("queue.workers", 10)
	viper.SetDefault("queue.concurrency", 5)
	viper.SetDefault("queue.max_retries", 3)
	viper.SetDefault("queue.retry_delay", 5)
	viper.SetDefault("queue.max_retry_delay", 300) // 5 minutes

	viper.SetDefault("monitoring.enabled", true)
	viper.SetDefault("monitoring.port", 9090)
	viper.SetDefault("monitoring.path", "/metrics")
	viper.SetDefault("monitoring.namespace", "freerange")

	viper.SetDefault("security.api_key_header", "X-API-Key")
	viper.SetDefault("security.rate_limit", 1000)
	viper.SetDefault("security.rate_limit_window", 3600)

	viper.SetDefault("providers.smtp.host", "")
	viper.SetDefault("providers.smtp.port", 587)
	viper.SetDefault("providers.sendgrid.api_key", "")

	// Configure viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/freerange/")

	// Enable environment variable override
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("FREERANGE")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal config
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	if len(c.Database.URLs) == 0 {
		return fmt.Errorf("database.urls cannot be empty")
	}

	if c.Redis.Host == "" {
		return fmt.Errorf("redis.host is required")
	}

	return nil
}
