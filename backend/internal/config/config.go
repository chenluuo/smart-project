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
	Server   ServerConfig
	Database DatabaseConfig
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
