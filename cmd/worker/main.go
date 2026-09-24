package main

import (
	"context"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/config"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/events"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/observability"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/repository"
	"github.com/Samuel-DA/open-banking-financial-insights/internal/service"
	"log/slog"
	"os/signal"
	"syscall"
	"time"
)

type noPublisher struct{}

func (noPublisher) PublishTransaction(context.Context, interface{}) error { return nil }
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
	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, "insights-worker-v1")
	if err != nil {
		slog.Error("kafka connection failed", "error", err)
		return
	}
	defer consumer.Close()
	svc := service.New(repo, nil)
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := svc.Expire(ctx); err != nil {
					slog.Error("consent expiry failed", "error", err)
				} else if n > 0 {
					slog.Info("consents expired", "count", n)
				}
			}
		}
	}()
	if err := consumer.Run(ctx, svc.ProcessTransaction); err != nil && ctx.Err() == nil {
		slog.Error("worker stopped", "error", err)
	}
}
