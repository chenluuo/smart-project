package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/config"
	httpserver "github.com/chenluuo/smart-project/backend/internal/http"
	"github.com/chenluuo/smart-project/backend/internal/platform/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg.Database)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("get database connection", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if cfg.Database.Migrate {
		if err := database.Migrate(context.Background(), sqlDB); err != nil {
			slog.Error("run database migrations", "error", err)
			os.Exit(1)
		}
	}

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           httpserver.NewRouter(cfg.Server.Mode, sqlDB),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("smart agriculture API started", "address", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case <-shutdownSignal.Done():
		slog.Info("shutting down API")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve API", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown", "error", err)
		os.Exit(1)
	}
}
