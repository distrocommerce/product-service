package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/distrocommerce/product-service/internal/config"
	"github.com/distrocommerce/product-service/internal/store"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("exit", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Ctrl+C during migrations cancels them (ctx).
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close() // runs after the server has shut down

	migrateCtx, cancelMigrate := context.WithTimeout(ctx, 30*time.Second)
	defer cancelMigrate()
	if err := st.Migrate(migrateCtx, cfg.SeedLocalData); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Info("migrations applied", "local_data", cfg.SeedLocalData)

	// True once the service can take traffic, false from shutdown on.
	var ready atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable) // shutting down: no DB call
			return
		}
		pingCtx, cancel := context.WithTimeout(r.Context(), 800*time.Millisecond) // < probe timeout (1 s)
		defer cancel()
		if err := st.Ping(pingCtx); err != nil {
			log.Warn("readiness: database unavailable", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	ready.Store(true)

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		stop() // a second signal now kills the process immediately
		log.Info("shutdown started")
	}

	// Fail readiness first, then give the gateway time to stop routing here.
	ready.Store(false)
	time.Sleep(cfg.DrainDelay)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close() // deadline passed: drop the remaining connections
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Info("shutdown complete")

	return nil
}
