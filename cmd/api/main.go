package main

import (
	"context"
	"errors"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/api"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/config"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/events"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/observability"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/repository"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/service"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	observability.SetupLogging()
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	repo, err := repository.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		return
	}
	defer repo.Close()
	publisher, err := events.NewKafkaPublisher(cfg.KafkaBrokers, cfg.KafkaTopic)
	if err != nil {
		slog.Error("kafka connection failed", "error", err)
		return
	}
	defer publisher.Close()
	shutdownTrace, err := observability.SetupTracing(ctx, cfg.OTLPEndpoint, "insights-api")
	if err != nil {
		slog.Warn("tracing disabled", "error", err)
	} else {
		defer shutdownTrace(context.Background())
	}
	svc := service.New(repo, publisher)
	srv := &http.Server{Addr: cfg.HTTPAddress, Handler: api.New(svc, repo), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("api listening", "address", cfg.HTTPAddress)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
