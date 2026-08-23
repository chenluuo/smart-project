package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
)

type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Auth          AuthConfig
	Internal      InternalConfig
	ObjectStorage ObjectStorageConfig
}

type ServerConfig struct {
	Port            string
	Mode            string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Migrate         bool
}

type AuthConfig struct {
	JWTSecret string
	Issuer    string
	TokenTTL  time.Duration
}

type InternalConfig struct {
	ServiceKey             string
	KnowledgeNotifyURL     string
	AgentAlertURL          string
	OutboxDispatchInterval time.Duration
	OutboxBatchSize        int
	AgentHTTPTimeout       time.Duration
}

type ObjectStorageConfig struct {
	Enabled          bool
	Endpoint         string
	PublicEndpoint   string
	AccessKey        string
	SecretKey        string
	Bucket           string
	Region           string
	Secure           bool
	MaxUploadBytes   int64
	SignedURLTimeout time.Duration
}

func Load() (Config, error) {
	maxOpen, err := intValue("DB_POOL_MAX_SIZE", 10)
	if err != nil {
		return Config{}, err
	}
	maxIdle, err := intValue("DB_POOL_MIN_IDLE", 2)
	if err != nil {
		return Config{}, err
	}
	migrate, err := boolValue("DB_MIGRATE", true)
	if err != nil {
		return Config{}, err
	}

	dsn, err := databaseDSN()
	if err != nil {
		return Config{}, err
	}
	tokenTTL, err := durationValue("JWT_TTL", 2*time.Hour)
	if err != nil {
		return Config{}, err
	}
	jwtSecret := value("JWT_SECRET", "dev-only-jwt-secret-change-me-32-chars")
	if len(jwtSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	internalServiceKey := value("INTERNAL_SERVICE_KEY", "dev-only-internal-service-key-change-me")
	if len(internalServiceKey) < 32 {
		return Config{}, fmt.Errorf("INTERNAL_SERVICE_KEY must contain at least 32 characters")
	}
	outboxDispatchInterval, err := durationValue("OUTBOX_DISPATCH_INTERVAL", 2*time.Second)
	if err != nil {
		return Config{}, err
	}
	outboxBatchSize, err := intValue("OUTBOX_BATCH_SIZE", 50)
	if err != nil || outboxBatchSize < 1 || outboxBatchSize > 500 {
		return Config{}, fmt.Errorf("OUTBOX_BATCH_SIZE must be between 1 and 500")
	}
	agentHTTPTimeout, err := durationValue("AGENT_HTTP_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	objectStorageEnabled, err := boolValue("OBJECT_STORAGE_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	objectStorageSecure, err := boolValue("MINIO_SECURE", false)
	if err != nil {
		return Config{}, err
	}
	maxUploadBytes, err := int64Value("KNOWLEDGE_MAX_UPLOAD_BYTES", 20*1024*1024)
	if err != nil || maxUploadBytes < 1 {
		return Config{}, fmt.Errorf("KNOWLEDGE_MAX_UPLOAD_BYTES must be a positive integer")
	}
	signedURLTimeout, err := durationValue("MINIO_SIGNED_URL_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Server: ServerConfig{
			Port:            value("SERVER_PORT", "8080"),
			Mode:            value("GIN_MODE", "debug"),
			ShutdownTimeout: 10 * time.Second,
		},
		Database: DatabaseConfig{
			DSN:             dsn,
			MaxOpenConns:    maxOpen,
			MaxIdleConns:    maxIdle,
			ConnMaxLifetime: 30 * time.Minute,
			Migrate:         migrate,
		},
		Auth: AuthConfig{
			JWTSecret: jwtSecret,
			Issuer:    value("JWT_ISSUER", "smart-agriculture-api"),
			TokenTTL:  tokenTTL,
		},
		Internal: InternalConfig{
			ServiceKey: internalServiceKey, KnowledgeNotifyURL: strings.TrimSpace(os.Getenv("KNOWLEDGE_NOTIFY_URL")),
			AgentAlertURL:          strings.TrimSpace(os.Getenv("AGENT_ALERT_URL")),
			OutboxDispatchInterval: outboxDispatchInterval, OutboxBatchSize: outboxBatchSize, AgentHTTPTimeout: agentHTTPTimeout,
		},
		ObjectStorage: ObjectStorageConfig{
			Enabled: objectStorageEnabled, Endpoint: value("MINIO_ENDPOINT", "localhost:9000"),
			PublicEndpoint: strings.TrimSpace(os.Getenv("MINIO_PUBLIC_ENDPOINT")),
			AccessKey:      value("MINIO_ACCESS_KEY", "minioadmin"), SecretKey: value("MINIO_SECRET_KEY", "minioadmin"),
			Bucket: value("MINIO_BUCKET", "knowledge"), Region: value("MINIO_REGION", "us-east-1"),
			Secure: objectStorageSecure, MaxUploadBytes: maxUploadBytes, SignedURLTimeout: signedURLTimeout,
		},
	}, nil
}

func databaseDSN() (string, error) {
	if dsn := os.Getenv("DB_DSN"); dsn != "" {
		return normalizeDSN(dsn)
	}

	username := value("DB_USERNAME", "smart_agriculture")
	password := value("DB_PASSWORD", "smart_agriculture")
	databaseURL := value("DB_URL", "jdbc:mysql://localhost:3307/smart_agriculture?useUnicode=true&characterEncoding=utf8&serverTimezone=UTC&allowPublicKeyRetrieval=true")
	databaseURL = strings.TrimPrefix(databaseURL, "jdbc:")
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Scheme != "mysql" || parsed.Host == "" || strings.TrimPrefix(parsed.Path, "/") == "" {
		return "", fmt.Errorf("DB_URL must be a MySQL URL: %q", databaseURL)
	}

	params := parsed.Query()
	params.Del("useUnicode")
	params.Del("characterEncoding")
	params.Del("serverTimezone")
	params.Del("allowPublicKeyRetrieval")
	params.Set("charset", "utf8mb4")
	params.Set("parseTime", "true")
	params.Set("loc", "UTC")
	params.Set("multiStatements", "true")
	return normalizeDSN(fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", username, password, parsed.Host, strings.TrimPrefix(parsed.Path, "/"), params.Encode()))
}

func normalizeDSN(dsn string) (string, error) {
	cfg, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("invalid DB_DSN: %w", err)
	}
	cfg.ParseTime = true
	cfg.MultiStatements = true
	if cfg.Loc == nil {
		cfg.Loc = time.UTC
	}
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	if _, ok := cfg.Params["charset"]; !ok {
		cfg.Params["charset"] = "utf8mb4"
	}
	return cfg.FormatDSN(), nil
}

func value(key, fallback string) string {
	if result := os.Getenv(key); result != "" {
		return result
	}
	return fallback
}

func intValue(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	result, err := strconv.Atoi(raw)
	if err != nil || result < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return result, nil
}

func int64Value(key string, fallback int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	result, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return result, nil
}

func boolValue(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	result, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return result, nil
}

func durationValue(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	result, err := time.ParseDuration(raw)
	if err != nil || result <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return result, nil
}
