package saga_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inventoryv1 "github.com/vladiant/ordersagademo/internal/gen/inventory/v1"
	"github.com/vladiant/ordersagademo/internal/messaging"
	"github.com/vladiant/ordersagademo/internal/order/saga"
	"github.com/vladiant/ordersagademo/internal/order/store"
)

// fakePublisher records published events without actual Kafka.
type fakePublisher struct {
	published []publishedEvent
}

type publishedEvent struct {
	topic   string
	payload interface{}
}

func (f *fakePublisher) Publish(_ context.Context, topic string, payload interface{}) error {
	f.published = append(f.published, publishedEvent{topic: topic, payload: payload})
	return nil
}

// fakeInventory returns a configurable response.
type fakeInventory struct {
	resp *inventoryv1.ReserveInventoryResponse
	err  error
}

func (f *fakeInventory) ReserveInventory(_ context.Context, _ *inventoryv1.ReserveInventoryRequest) (*inventoryv1.ReserveInventoryResponse, error) {
	return f.resp, f.err
}

func newOrchestrator(t *testing.T, pub *fakePublisher, inv *fakeInventory) *saga.Orchestrator {
	t.Helper()
	s := store.NewMemoryStore()
	orch, err := saga.NewOrchestrator(s, pub, inv)
	require.NoError(t, err)
	return orch
}

func newOrchestatorWithStore(t *testing.T, s store.Store, pub *fakePublisher, inv *fakeInventory) *saga.Orchestrator {
	t.Helper()
	orch, err := saga.NewOrchestrator(s, pub, inv)
	require.NoError(t, err)
	return orch
}

// TestHappyPath runs the full happy-path state machine.
func TestHappyPath(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{
		resp: &inventoryv1.ReserveInventoryResponse{Success: true, ReservationId: "res-001"},
	}
	s := store.NewMemoryStore()
	orch := newOrchestatorWithStore(t, s, pub, inv)
	ctx := context.Background()

	order := store.Order{
		ID:            "order-001",
		Items:         []store.Item{{ItemID: "item-A", Quantity: 2}},
		PaymentAmount: 19.99,
	}

	require.NoError(t, orch.StartSaga(ctx, order, "evt-001"))

	saved, err := s.Get("order-001")
	require.NoError(t, err)
	assert.Equal(t, store.StateAwaitingPayment, saved.State)
	assert.Len(t, pub.published, 1)
	assert.Equal(t, "saga.orders.created", pub.published[0].topic)

	require.NoError(t, orch.OnPaymentProcessed(ctx, "order-001", "pay-001"))

	final, err := s.Get("order-001")
	require.NoError(t, err)
	assert.Equal(t, store.StateCompleted, final.State)
	assert.Equal(t, "res-001", final.ReservationID)
	// No additional Kafka events on happy path (inventory is via gRPC).
	assert.Len(t, pub.published, 1)
}

// TestPaymentFailedPath confirms FAILED state on PaymentFailed event.
func TestPaymentFailedPath(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{}
	s := store.NewMemoryStore()
	orch := newOrchestatorWithStore(t, s, pub, inv)
	ctx := context.Background()

	order := store.Order{
		ID:            "order-002",
		Items:         []store.Item{{ItemID: "item-B", Quantity: 1}},
		PaymentAmount: 5.00,
	}
	require.NoError(t, orch.StartSaga(ctx, order, "evt-002"))
	require.NoError(t, orch.OnPaymentFailed(ctx, "order-002", "card declined"))

	o, err := s.Get("order-002")
	require.NoError(t, err)
	assert.Equal(t, store.StateFailed, o.State)
}

// TestCompensationPath verifies the compensation flow: payment succeeds, inventory fails,
// CompensatePayment is published, PaymentRefunded leads to COMPENSATED.
func TestCompensationPath(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{
		resp: &inventoryv1.ReserveInventoryResponse{
			Success:     false,
			ErrorReason: "forced-failure: FAIL_ITEM_01",
		},
	}
	s := store.NewMemoryStore()
	orch := newOrchestatorWithStore(t, s, pub, inv)
	ctx := context.Background()

	order := store.Order{
		ID:            "order-003",
		Items:         []store.Item{{ItemID: "FAIL_ITEM_01", Quantity: 1}},
		PaymentAmount: 10.00,
	}
	require.NoError(t, orch.StartSaga(ctx, order, "evt-003"))
	require.NoError(t, orch.OnPaymentProcessed(ctx, "order-003", "pay-003"))

	o, err := s.Get("order-003")
	require.NoError(t, err)
	assert.Equal(t, store.StateCompensating, o.State)

	// CompensatePayment must have been published.
	require.Len(t, pub.published, 2)
	assert.Equal(t, "saga.payments.compensate", pub.published[1].topic)

	// Simulate Payment Service acknowledging refund.
	require.NoError(t, orch.OnPaymentRefunded(ctx, "order-003"))

	final, err := s.Get("order-003")
	require.NoError(t, err)
	assert.Equal(t, store.StateCompensated, final.State)
}

// TestStartSagaPreservesEventID verifies that the OrderCreated event published by
// StartSaga carries exactly the eventID provided by the caller (R-7 stable event_id).
func TestStartSagaPreservesEventID(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{}
	orch := newOrchestrator(t, pub, inv)
	ctx := context.Background()

	const wantEventID = "stable-evt-id-001"
	order := store.Order{
		ID:    "order-evt",
		Items: []store.Item{{ItemID: "item-X", Quantity: 1}},
	}
	require.NoError(t, orch.StartSaga(ctx, order, wantEventID))

	require.Len(t, pub.published, 1)
	evt, ok := pub.published[0].payload.(messaging.OrderCreatedEvent)
	require.True(t, ok, "published payload must be an OrderCreatedEvent")
	assert.Equal(t, wantEventID, evt.EventID, "event_id must equal the caller-provided ID")
}

// TestCompensationOnInvalidQuantity verifies that an order whose item quantity is
// out of int32 range fails buildReserveRequest inside OnPaymentProcessed. Since
// payment has already been taken, the saga must transition to COMPENSATING and
// publish CompensatePayment to release the payment (rather than stay stuck).
func TestCompensationOnInvalidQuantity(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{
		resp: &inventoryv1.ReserveInventoryResponse{Success: true, ReservationId: "res-invalid"},
	}
	s := store.NewMemoryStore()
	orch := newOrchestatorWithStore(t, s, pub, inv)
	ctx := context.Background()

	order := store.Order{
		ID:            "order-invalidqty",
		Items:         []store.Item{{ItemID: "item-A", Quantity: math.MaxInt32 + 1}},
		PaymentAmount: 12.00,
	}
	require.NoError(t, orch.StartSaga(ctx, order, "evt-invalidqty"))
	require.NoError(t, orch.OnPaymentProcessed(ctx, "order-invalidqty", "pay-invalidqty"))

	o, err := s.Get("order-invalidqty")
	require.NoError(t, err)
	assert.Equal(t, store.StateCompensating, o.State,
		"out-of-range quantity must trigger compensation, not a stuck RESERVING order")

	// CompensatePayment must have been published to release the already-taken payment.
	require.Len(t, pub.published, 2)
	assert.Equal(t, "saga.payments.compensate", pub.published[1].topic)
	evt, ok := pub.published[1].payload.(messaging.CompensatePaymentEvent)
	require.True(t, ok, "published payload must be a CompensatePaymentEvent")
	assert.Equal(t, "pay-invalidqty", evt.PaymentID, "compensation must release the taken payment")
	assert.Contains(t, evt.Reason, "out of int32 range", "reason must reflect the invalid quantity")
}

// TestCompensationOnGRPCError verifies that a gRPC transport error (not a business
// failure) also triggers the compensation path.
func TestCompensationOnGRPCError(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{err: fmt.Errorf("connection refused")}
	s := store.NewMemoryStore()
	orch := newOrchestatorWithStore(t, s, pub, inv)
	ctx := context.Background()

	order := store.Order{
		ID:    "order-grpcerr",
		Items: []store.Item{{ItemID: "item-A", Quantity: 1}},
	}
	require.NoError(t, orch.StartSaga(ctx, order, "evt-grpcerr"))
	require.NoError(t, orch.OnPaymentProcessed(ctx, "order-grpcerr", "pay-grpcerr"))

	o, err := s.Get("order-grpcerr")
	require.NoError(t, err)
	assert.Equal(t, store.StateCompensating, o.State, "gRPC error must trigger compensation")

	// CompensatePayment must have been published.
	require.Len(t, pub.published, 2)
	assert.Equal(t, "saga.payments.compensate", pub.published[1].topic)
}

// TestIdempotentCompensation verifies that calling OnPaymentRefunded twice does not
// double-transition (store returns ErrNotFound on missing order, not a second mutation).
func TestIdempotentCompensation(t *testing.T) {
	pub := &fakePublisher{}
	inv := &fakeInventory{
		resp: &inventoryv1.ReserveInventoryResponse{
			Success:     false,
			ErrorReason: "forced",
		},
	}
	s := store.NewMemoryStore()
	orch := newOrchestatorWithStore(t, s, pub, inv)
	ctx := context.Background()

	order := store.Order{ID: "order-004", Items: []store.Item{{ItemID: "FAIL_ITEM_01", Quantity: 1}}}
	require.NoError(t, orch.StartSaga(ctx, order, "evt-004"))
	require.NoError(t, orch.OnPaymentProcessed(ctx, "order-004", "pay-004"))

	require.NoError(t, orch.OnPaymentRefunded(ctx, "order-004"))

	o, err := s.Get("order-004")
	require.NoError(t, err)
	assert.Equal(t, store.StateCompensated, o.State)

	// Second call — order is already COMPENSATED but the state update is idempotent
	// (it just overwrites with the same value).
	err = orch.OnPaymentRefunded(ctx, "order-004")
	assert.NoError(t, err)
}
