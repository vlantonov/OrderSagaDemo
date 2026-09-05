// Package messaging provides shared Kafka event envelope types and helpers.
package messaging

import (
	"encoding/json"
	"fmt"
	"time"
)

// Envelope is the common wrapper present in every event.
// Producers must populate all fields; consumers validate them.
type Envelope struct {
	EventType    string       `json:"event_type"`
	EventID      string       `json:"event_id"`       // idempotency key (UUID, generated once per message)
	Timestamp    string       `json:"timestamp"`      // RFC3339
	TraceContext TraceContext `json:"trace_context"`
}

// TraceContext carries the W3C traceparent header for cross-Kafka propagation.
type TraceContext struct {
	Traceparent string `json:"traceparent"`
}

// NewEnvelope creates an Envelope with the current timestamp.
func NewEnvelope(eventType, eventID string) Envelope {
	return Envelope{
		EventType: eventType,
		EventID:   eventID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// OrderCreatedEvent is published to saga.orders.created.
type OrderCreatedEvent struct {
	Envelope
	OrderID       string         `json:"order_id"`
	Items         []ItemQuantity `json:"items"`
	PaymentAmount float64        `json:"payment_amount"`
}

// ItemQuantity pairs an item identifier with a requested quantity.
type ItemQuantity struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// PaymentProcessedEvent is published to saga.payments.processed.
type PaymentProcessedEvent struct {
	Envelope
	OrderID   string `json:"order_id"`
	PaymentID string `json:"payment_id"`
}

// PaymentFailedEvent is published to saga.payments.failed.
type PaymentFailedEvent struct {
	Envelope
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

// CompensatePaymentEvent is published to saga.payments.compensate.
type CompensatePaymentEvent struct {
	Envelope
	OrderID   string `json:"order_id"`
	PaymentID string `json:"payment_id"`
	Reason    string `json:"reason"`
}

// PaymentRefundedEvent is published to saga.payments.refunded.
type PaymentRefundedEvent struct {
	Envelope
	OrderID   string `json:"order_id"`
	PaymentID string `json:"payment_id"`
	RefundID  string `json:"refund_id"`
}

// InventoryReservedEvent is published to saga.inventory.reserved (audit).
type InventoryReservedEvent struct {
	Envelope
	OrderID       string `json:"order_id"`
	ReservationID string `json:"reservation_id"`
}

// InventoryReleasedEvent is published to saga.inventory.released (audit).
type InventoryReleasedEvent struct {
	Envelope
	OrderID       string `json:"order_id"`
	ReservationID string `json:"reservation_id"`
}

// Decode unmarshals JSON bytes into dst and returns the embedded Envelope.
func Decode(data []byte, dst interface{}) (Envelope, error) {
	if err := json.Unmarshal(data, dst); err != nil {
		return Envelope{}, fmt.Errorf("messaging decode: %w", err)
	}
	// Extract envelope from the decoded struct via a second unmarshal pass.
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, fmt.Errorf("messaging decode envelope: %w", err)
	}
	return env, nil
}
