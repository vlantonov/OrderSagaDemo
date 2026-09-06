// Package saga implements the order saga state machine and compensation logic.
// The Order Service acts as the orchestrator: it drives every saga step and
// triggers compensation centrally when a step fails.
package saga

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	inventoryv1 "github.com/vladiant/ordersagademo/internal/gen/inventory/v1"
	"github.com/vladiant/ordersagademo/internal/messaging"
	"github.com/vladiant/ordersagademo/internal/order/store"
)

const tracerName = "ordersagademo/order/saga"

// Publisher sends a Kafka event. It returns an error if delivery fails.
type Publisher interface {
	Publish(ctx context.Context, topic string, payload interface{}) error
}

// InventoryClient is the gRPC client interface for the Inventory Service.
type InventoryClient interface {
	ReserveInventory(ctx context.Context, req *inventoryv1.ReserveInventoryRequest) (*inventoryv1.ReserveInventoryResponse, error)
}

// Orchestrator drives the saga state machine for a single order.
type Orchestrator struct {
	store     store.Store
	publisher Publisher
	inventory InventoryClient
	tracer    trace.Tracer
	meter     metric.Meter

	ordersTotal       metric.Int64Counter
	stepDuration      metric.Float64Histogram
	compensationTotal metric.Int64Counter
}

// NewOrchestrator wires up an Orchestrator with the provided dependencies.
func NewOrchestrator(s store.Store, p Publisher, inv InventoryClient) (*Orchestrator, error) {
	meter := otel.GetMeterProvider().Meter(tracerName)

	ordersTotal, err := meter.Int64Counter("saga_orders_total",
		metric.WithDescription("Order outcomes by status"))
	if err != nil {
		return nil, fmt.Errorf("saga metric saga_orders_total: %w", err)
	}
	stepDuration, err := meter.Float64Histogram("saga_step_duration_seconds",
		metric.WithDescription("Per-step saga latency"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("saga metric saga_step_duration_seconds: %w", err)
	}
	compensationTotal, err := meter.Int64Counter("saga_compensation_total",
		metric.WithDescription("Rollback count by reason"))
	if err != nil {
		return nil, fmt.Errorf("saga metric saga_compensation_total: %w", err)
	}

	return &Orchestrator{
		store:             s,
		publisher:         p,
		inventory:         inv,
		tracer:            otel.Tracer(tracerName),
		meter:             meter,
		ordersTotal:       ordersTotal,
		stepDuration:      stepDuration,
		compensationTotal: compensationTotal,
	}, nil
}

// StartSaga persists the order in PENDING state, publishes OrderCreated, and
// transitions to AWAITING_PAYMENT. eventID must be generated once before any
// retry loop (R-7).
func (o *Orchestrator) StartSaga(ctx context.Context, order store.Order, eventID string) error {
	ctx, span := o.tracer.Start(ctx, "saga.StartSaga",
		trace.WithAttributes(attribute.String("order.id", order.ID)))
	defer span.End()

	order.State = store.StatePending
	if err := o.store.Save(order); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "save order")
		return fmt.Errorf("save order: %w", err)
	}

	items := make([]messaging.ItemQuantity, len(order.Items))
	for i, it := range order.Items {
		items[i] = messaging.ItemQuantity{ItemID: it.ItemID, Quantity: it.Quantity}
	}
	evt := messaging.OrderCreatedEvent{
		Envelope:      messaging.NewEnvelope("OrderCreated", eventID),
		OrderID:       order.ID,
		Items:         items,
		PaymentAmount: order.PaymentAmount,
	}

	if err := o.publisher.Publish(ctx, "saga.orders.created", evt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish OrderCreated")
		return fmt.Errorf("publish OrderCreated: %w", err)
	}

	if err := o.store.UpdateState(order.ID, store.StateAwaitingPayment); err != nil {
		return fmt.Errorf("transition to AWAITING_PAYMENT: %w", err)
	}

	slog.InfoContext(ctx, "saga started",
		"order_id", order.ID,
		"state", store.StateAwaitingPayment,
	)
	return nil
}

// OnPaymentProcessed handles a PaymentProcessed event: transitions to RESERVING,
// calls ReserveInventory, and either completes or begins compensation.
func (o *Orchestrator) OnPaymentProcessed(ctx context.Context, orderID, paymentID string) error {
	ctx, span := o.tracer.Start(ctx, "saga.OnPaymentProcessed",
		trace.WithAttributes(
			attribute.String("order.id", orderID),
			attribute.String("payment.id", paymentID),
		))
	defer span.End()

	if err := o.store.SetPaymentID(orderID, paymentID); err != nil {
		return fmt.Errorf("set payment id: %w", err)
	}
	if err := o.store.UpdateState(orderID, store.StateReserving); err != nil {
		return fmt.Errorf("transition to RESERVING: %w", err)
	}

	order, err := o.store.Get(orderID)
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	req, err := buildReserveRequest(order)
	if err != nil {
		// Unfulfillable request (e.g. quantity out of range); payment already
		// taken, so compensate rather than leaving the order stuck in RESERVING.
		span.RecordError(err)
		slog.WarnContext(ctx, "invalid reservation request — compensating",
			"order_id", orderID,
			"error", err.Error(),
		)
		return o.beginCompensation(ctx, orderID, paymentID, fmt.Sprintf("invalid request: %v", err))
	}
	resp, err := o.inventory.ReserveInventory(ctx, req)
	if err != nil {
		// Network / infrastructure error — enter compensation immediately.
		span.RecordError(err)
		return o.beginCompensation(ctx, orderID, paymentID, fmt.Sprintf("grpc error: %v", err))
	}

	if !resp.GetSuccess() {
		reason := resp.GetErrorReason()
		span.SetAttributes(attribute.String("inventory.error", reason))
		slog.WarnContext(ctx, "inventory reservation failed — compensating",
			"order_id", orderID,
			"reason", reason,
		)
		return o.beginCompensation(ctx, orderID, paymentID, reason)
	}

	reservationID := resp.GetReservationId()
	if err := o.store.SetReservationID(orderID, reservationID); err != nil {
		return fmt.Errorf("set reservation id: %w", err)
	}
	if err := o.store.UpdateState(orderID, store.StateCompleted); err != nil {
		return fmt.Errorf("transition to COMPLETED: %w", err)
	}

	o.ordersTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "completed")))
	slog.InfoContext(ctx, "saga completed",
		"order_id", orderID,
		"reservation_id", reservationID,
	)
	return nil
}

// OnPaymentFailed handles a PaymentFailed event: transitions directly to FAILED.
func (o *Orchestrator) OnPaymentFailed(ctx context.Context, orderID, reason string) error {
	ctx, span := o.tracer.Start(ctx, "saga.OnPaymentFailed",
		trace.WithAttributes(attribute.String("order.id", orderID)))
	defer span.End()

	if err := o.store.UpdateState(orderID, store.StateFailed); err != nil {
		return fmt.Errorf("transition to FAILED: %w", err)
	}

	o.ordersTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "failed")))
	slog.WarnContext(ctx, "saga failed — payment rejected",
		"order_id", orderID,
		"reason", reason,
	)
	return nil
}

// OnPaymentRefunded handles a PaymentRefunded event: transitions to COMPENSATED.
func (o *Orchestrator) OnPaymentRefunded(ctx context.Context, orderID string) error {
	ctx, span := o.tracer.Start(ctx, "saga.OnPaymentRefunded",
		trace.WithAttributes(attribute.String("order.id", orderID)))
	defer span.End()

	if err := o.store.UpdateState(orderID, store.StateCompensated); err != nil {
		return fmt.Errorf("transition to COMPENSATED: %w", err)
	}

	o.ordersTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "compensated")))
	slog.InfoContext(ctx, "saga compensated",
		"order_id", orderID,
		"state", store.StateCompensated,
	)
	return nil
}

// beginCompensation transitions to COMPENSATING and publishes CompensatePayment.
func (o *Orchestrator) beginCompensation(ctx context.Context, orderID, paymentID, reason string) error {
	if err := o.store.UpdateState(orderID, store.StateCompensating); err != nil {
		return fmt.Errorf("transition to COMPENSATING: %w", err)
	}

	eventID := newUUID()
	evt := messaging.CompensatePaymentEvent{
		Envelope:  messaging.NewEnvelope("CompensatePayment", eventID),
		OrderID:   orderID,
		PaymentID: paymentID,
		Reason:    reason,
	}
	if err := o.publisher.Publish(ctx, "saga.payments.compensate", evt); err != nil {
		return fmt.Errorf("publish CompensatePayment: %w", err)
	}

	o.compensationTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
	slog.WarnContext(ctx, "compensation initiated",
		"order_id", orderID,
		"payment_id", paymentID,
		"reason", reason,
	)
	return nil
}

func buildReserveRequest(order store.Order) (*inventoryv1.ReserveInventoryRequest, error) {
	items := make([]*inventoryv1.ItemQuantity, len(order.Items))
	for i, it := range order.Items {
		qty := it.Quantity
		if qty < 0 || qty > math.MaxInt32 {
			return nil, fmt.Errorf("item %q quantity %d out of int32 range", it.ItemID, qty)
		}
		items[i] = &inventoryv1.ItemQuantity{
			ItemId: it.ItemID,
			// qty is bounds-checked to [0, math.MaxInt32] above; gosec v2.20 (G115)
			// cannot see the guard, so the conversion is safe despite the finding.
			Quantity: int32(qty), //nolint:gosec // G115: guarded by the range check above
		}
	}
	return &inventoryv1.ReserveInventoryRequest{
		OrderId: order.ID,
		Items:   items,
	}, nil
}
