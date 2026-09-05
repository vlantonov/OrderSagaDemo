// Package grpc provides the gRPC server implementation for the Inventory Service.
package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/google/uuid"
	"encoding/json"

	inventoryv1 "github.com/vladiant/ordersagademo/internal/gen/inventory/v1"
	"github.com/vladiant/ordersagademo/internal/inventory/store"
	"github.com/vladiant/ordersagademo/internal/messaging"
)

const tracerName = "ordersagademo/inventory/grpc"

// Server implements inventoryv1.InventoryServiceServer.
type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	stock         store.Store
	failPrefix    string // INVENTORY_FAIL_ITEM_PREFIX (R-6)
	kafkaWriter   *kafkago.Writer
	grpcHandled   metric.Int64Counter
}

// NewServer constructs a Server.
// failPrefix: if non-empty, any item_id starting with this prefix triggers forced failure.
// brokers: Kafka broker addresses for audit events.
func NewServer(stock store.Store, failPrefix string, brokers []string) (*Server, error) {
	meter := otel.GetMeterProvider().Meter(tracerName)
	grpcHandled, err := meter.Int64Counter("grpc_server_handled_total",
		metric.WithDescription("gRPC server calls by method and code"))
	if err != nil {
		return nil, fmt.Errorf("inventory grpc metric: %w", err)
	}

	w := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafkago.RequireOne,
	}

	return &Server{
		stock:       stock,
		failPrefix:  failPrefix,
		kafkaWriter: w,
		grpcHandled: grpcHandled,
	}, nil
}

// ReserveInventory checks stock and reserves items, or returns failure for forced-failure items (R-6).
func (s *Server) ReserveInventory(ctx context.Context, req *inventoryv1.ReserveInventoryRequest) (*inventoryv1.ReserveInventoryResponse, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "InventoryService.ReserveInventory",
		// otelgrpc interceptor already starts the span; this is a child for business logic.
	)
	defer span.End()

	span.SetAttributes(attribute.String("order.id", req.GetOrderId()))

	// Forced-failure check (R-6).
	if s.failPrefix != "" {
		for _, it := range req.GetItems() {
			if strings.HasPrefix(it.GetItemId(), s.failPrefix) {
				reason := fmt.Sprintf("forced-failure: %s", it.GetItemId())
				slog.WarnContext(ctx, "inventory forced failure", "order_id", req.GetOrderId(), "item_id", it.GetItemId())
				s.grpcHandled.Add(ctx, 1, metric.WithAttributes(
					attribute.String("grpc_method", "ReserveInventory"),
					attribute.String("grpc_code", "FORCED_FAILURE"),
				))
				return &inventoryv1.ReserveInventoryResponse{
					Success:     false,
					ErrorReason: reason,
				}, nil
			}
		}
	}

	items := make([]store.Item, len(req.GetItems()))
	for i, it := range req.GetItems() {
		items[i] = store.Item{ItemID: it.GetItemId(), Quantity: int(it.GetQuantity())}
	}

	reservationID, err := s.stock.ReserveStock(req.GetOrderId(), items)
	if err != nil {
		slog.WarnContext(ctx, "reserve stock failed", "order_id", req.GetOrderId(), "error", err)
		s.grpcHandled.Add(ctx, 1, metric.WithAttributes(
			attribute.String("grpc_method", "ReserveInventory"),
			attribute.String("grpc_code", "INSUFFICIENT_STOCK"),
		))
		return &inventoryv1.ReserveInventoryResponse{
			Success:     false,
			ErrorReason: err.Error(),
		}, nil
	}

	s.grpcHandled.Add(ctx, 1, metric.WithAttributes(
		attribute.String("grpc_method", "ReserveInventory"),
		attribute.String("grpc_code", "OK"),
	))
	slog.InfoContext(ctx, "inventory reserved",
		"order_id", req.GetOrderId(),
		"reservation_id", reservationID,
	)

	// Publish audit event (R-2: inject trace into headers).
	s.publishAudit(ctx, "saga.inventory.reserved", messaging.InventoryReservedEvent{
		Envelope:      messaging.NewEnvelope("InventoryReserved", uuid.New().String()),
		OrderID:       req.GetOrderId(),
		ReservationID: reservationID,
	})

	return &inventoryv1.ReserveInventoryResponse{
		Success:       true,
		ReservationId: reservationID,
	}, nil
}

// ReleaseInventory releases a prior reservation.
func (s *Server) ReleaseInventory(ctx context.Context, req *inventoryv1.ReleaseInventoryRequest) (*inventoryv1.ReleaseInventoryResponse, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "InventoryService.ReleaseInventory")
	defer span.End()

	if err := s.stock.ReleaseStock(req.GetOrderId(), req.GetReservationId()); err != nil {
		slog.WarnContext(ctx, "release stock failed", "order_id", req.GetOrderId(), "error", err)
		return &inventoryv1.ReleaseInventoryResponse{
			Success:     false,
			ErrorReason: err.Error(),
		}, nil
	}

	slog.InfoContext(ctx, "inventory released",
		"order_id", req.GetOrderId(),
		"reservation_id", req.GetReservationId(),
	)

	s.publishAudit(ctx, "saga.inventory.released", messaging.InventoryReleasedEvent{
		Envelope:      messaging.NewEnvelope("InventoryReleased", uuid.New().String()),
		OrderID:       req.GetOrderId(),
		ReservationID: req.GetReservationId(),
	})

	return &inventoryv1.ReleaseInventoryResponse{Success: true}, nil
}

func (s *Server) publishAudit(ctx context.Context, topic string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "marshal audit event", "error", err)
		return
	}
	headers := make([]kafkago.Header, 0, 2)
	messaging.InjectTrace(ctx, &headers)
	if err := s.kafkaWriter.WriteMessages(ctx, kafkago.Message{
		Topic:   topic,
		Value:   data,
		Headers: headers,
	}); err != nil {
		slog.ErrorContext(ctx, "write audit event", "topic", topic, "error", err)
	}
}

// Close shuts down the Kafka writer.
func (s *Server) Close() error {
	return s.kafkaWriter.Close()
}
