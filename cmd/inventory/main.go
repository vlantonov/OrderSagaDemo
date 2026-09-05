// Inventory Service entry point: wires config → telemetry → stock store → gRPC server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	inventoryv1 "github.com/vladiant/ordersagademo/internal/gen/inventory/v1"
	inventorygrpc "github.com/vladiant/ordersagademo/internal/inventory/grpc"
	"github.com/vladiant/ordersagademo/internal/inventory/store"
	"github.com/vladiant/ordersagademo/internal/telemetry"
)

func main() {
	if err := run(); err != nil {
		slog.Error("inventory-service fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config()

	shutdown, err := telemetry.Init(ctx, "inventory-service", "0.1.0", cfg.otlpEndpoint)
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

	stockStore := store.NewMemoryStore()

	invServer, err := inventorygrpc.NewServer(stockStore, cfg.failPrefix, cfg.kafkaBrokers)
	if err != nil {
		return fmt.Errorf("inventory grpc server: %w", err)
	}
	defer invServer.Close()

	// Register OTel server interceptor (R-3).
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	inventoryv1.RegisterInventoryServiceServer(grpcServer, invServer)

	lis, err := net.Listen("tcp", cfg.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.grpcAddr, err)
	}

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	slog.Info("inventory-service starting",
		"addr", cfg.grpcAddr,
		"fail_prefix", cfg.failPrefix,
	)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}

type serviceConfig struct {
	grpcAddr     string
	kafkaBrokers []string
	otlpEndpoint string
	failPrefix   string
}

func config() serviceConfig {
	return serviceConfig{
		grpcAddr:     envOr("GRPC_ADDR", ":9090"),
		kafkaBrokers: strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ","),
		otlpEndpoint: envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		// INVENTORY_FAIL_ITEM_PREFIX: any item_id starting with this prefix triggers
		// the forced-failure compensation path. Set to empty string to disable (R-6).
		failPrefix: envOr("INVENTORY_FAIL_ITEM_PREFIX", "FAIL_"),
	}
}

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Ensure inventory service has enough time to flush telemetry before exit.
var _ = time.Second
