// Order Service entry point: wires config → telemetry → store → saga → HTTP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventoryv1 "github.com/vladiant/ordersagademo/internal/gen/inventory/v1"
	"github.com/vladiant/ordersagademo/internal/order/api"
	orderkafka "github.com/vladiant/ordersagademo/internal/order/kafka"
	"github.com/vladiant/ordersagademo/internal/order/saga"
	"github.com/vladiant/ordersagademo/internal/order/store"
	"github.com/vladiant/ordersagademo/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("order-service fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config()

	shutdown, err := telemetry.Init(ctx, "order-service", "0.1.0", cfg.otlpEndpoint)
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

	// gRPC client for Inventory Service with OTel interceptor (R-3).
	conn, err := grpc.NewClient(cfg.inventoryAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return fmt.Errorf("grpc dial inventory: %w", err)
	}
	defer conn.Close()
	invClient := inventoryv1.NewInventoryServiceClient(conn)

	orderStore := store.NewMemoryStore()

	producer, err := orderkafka.NewProducer(cfg.kafkaBrokers)
	if err != nil {
		return fmt.Errorf("kafka producer: %w", err)
	}
	defer producer.Close()

	invAdapter := &inventoryClientAdapter{client: invClient}
	orch, err := saga.NewOrchestrator(orderStore, producer, invAdapter)
	if err != nil {
		return fmt.Errorf("orchestrator: %w", err)
	}

	consumer, err := orderkafka.NewConsumer(cfg.kafkaBrokers, cfg.consumerGroup, orch)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}
	defer consumer.Close()

	go consumer.Run(ctx)

	mux := http.NewServeMux()
	handler := api.NewHandler(orch)
	handler.ServeHTTP(mux)

	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sCtx)
	}()

	slog.Info("order-service starting", "addr", cfg.httpAddr, "inventory", cfg.inventoryAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

type serviceConfig struct {
	httpAddr      string
	kafkaBrokers  []string
	otlpEndpoint  string
	inventoryAddr string
	consumerGroup string
}

func config() serviceConfig {
	return serviceConfig{
		httpAddr:      envOr("HTTP_ADDR", ":8080"),
		kafkaBrokers:  strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		otlpEndpoint:  envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		inventoryAddr: envOr("INVENTORY_GRPC_ADDR", "localhost:9090"),
		consumerGroup: envOr("KAFKA_CONSUMER_GROUP", "order-service"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// inventoryClientAdapter adapts the generated gRPC client to the saga.InventoryClient interface.
type inventoryClientAdapter struct {
	client inventoryv1.InventoryServiceClient
}

func (a *inventoryClientAdapter) ReserveInventory(ctx context.Context, req *inventoryv1.ReserveInventoryRequest) (*inventoryv1.ReserveInventoryResponse, error) {
	return a.client.ReserveInventory(ctx, req)
}
