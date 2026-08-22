package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"DB_DSN", "DB_URL", "DB_USERNAME", "DB_PASSWORD", "DB_POOL_MAX_SIZE", "DB_POOL_MIN_IDLE", "DB_MIGRATE", "SERVER_PORT", "GIN_MODE"} {
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
