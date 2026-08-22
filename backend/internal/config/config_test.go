package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"DB_DSN", "DB_URL", "DB_USERNAME", "DB_PASSWORD", "DB_POOL_MAX_SIZE", "DB_POOL_MIN_IDLE", "DB_MIGRATE", "SERVER_PORT", "GIN_MODE", "JWT_SECRET", "JWT_ISSUER", "JWT_TTL"} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != "8080" {
		t.Fatalf("server port = %q, want 8080", cfg.Server.Port)
	}
	if !strings.Contains(cfg.Database.DSN, "@tcp(localhost:3307)/smart_agriculture") {
		t.Fatalf("unexpected default DSN %q", cfg.Database.DSN)
	}
	if !strings.Contains(cfg.Database.DSN, "parseTime=true") || !strings.Contains(cfg.Database.DSN, "multiStatements=true") {
		t.Fatalf("default DSN is missing required options: %q", cfg.Database.DSN)
	}
	if cfg.Auth.TokenTTL != 2*time.Hour || cfg.Auth.Issuer != "smart-agriculture-api" {
		t.Fatalf("unexpected auth defaults: %+v", cfg.Auth)
	}
}

func TestInvalidJWTConfiguration(t *testing.T) {
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want short JWT secret error")
	}

	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("JWT_TTL", "never")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid JWT TTL error")
	}
}

func TestDBDSNTakesPrecedence(t *testing.T) {
	t.Setenv("DB_DSN", "user:secret@tcp(db:3306)/app?parseTime=true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, option := range []string{"parseTime=true", "multiStatements=true", "charset=utf8mb4"} {
		if !strings.Contains(cfg.Database.DSN, option) {
			t.Fatalf("DSN %q is missing %s", cfg.Database.DSN, option)
		}
	}
}

func TestInvalidDBDSN(t *testing.T) {
	t.Setenv("DB_DSN", "://not-a-dsn")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid DSN error")
	}
}
