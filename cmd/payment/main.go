// Payment Service entry point: wires config → telemetry → ledger → kafka worker.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	paymentkafka "github.com/vladiant/ordersagademo/internal/payment/kafka"
	"github.com/vladiant/ordersagademo/internal/payment/ledger"
	"github.com/vladiant/ordersagademo/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("payment-service fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config()

	shutdown, err := telemetry.Init(ctx, "payment-service", "0.1.0", cfg.otlpEndpoint)
	if err != nil {
		return fmt.Errorf("telemetry init: %w", err)
	}
	defer func() {
		sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(sCtx); err != nil {
			slog.Error("telemetry shutdown", "error", err)
		}
	}()

	l := ledger.NewMemoryLedger()

	worker, err := paymentkafka.NewWorker(cfg.kafkaBrokers, cfg.consumerGroup, l)
	if err != nil {
		return fmt.Errorf("kafka worker: %w", err)
	}
	defer worker.Close()

	slog.Info("payment-service starting", "brokers", cfg.kafkaBrokers)
	worker.Run(ctx)
	return nil
}

type serviceConfig struct {
	kafkaBrokers  []string
	otlpEndpoint  string
	consumerGroup string
}

func config() serviceConfig {
	return serviceConfig{
		kafkaBrokers:  strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		otlpEndpoint:  envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		consumerGroup: envOr("KAFKA_CONSUMER_GROUP", "payment-service"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
