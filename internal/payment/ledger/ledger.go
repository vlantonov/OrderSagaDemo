// Package ledger defines the PaymentLedger interface and in-memory implementation.
package ledger

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// ErrAlreadyProcessed is returned when the payment for an order was already processed.
var ErrAlreadyProcessed = errors.New("payment already processed")

// ErrAlreadyRefunded is returned when a refund for the payment was already issued.
var ErrAlreadyRefunded = errors.New("payment already refunded")

// ErrNotFound is returned when no payment record exists.
var ErrNotFound = errors.New("payment not found")

// Ledger is the read/write interface for payment records.
type Ledger interface {
	ProcessPayment(orderID string, amount float64) (paymentID string, err error)
	RefundPayment(orderID, paymentID string) (refundID string, err error)
}

type record struct {
	paymentID string
	refundID  string // empty until refunded
}

// MemoryLedger is a thread-safe in-memory payment ledger.
type MemoryLedger struct {
	mu      sync.Mutex
	records map[string]record // keyed on orderID
}

// NewMemoryLedger returns an initialised MemoryLedger.
func NewMemoryLedger() *MemoryLedger {
	return &MemoryLedger{records: make(map[string]record)}
}

// ProcessPayment records a new payment and returns the generated paymentID.
// Returns ErrAlreadyProcessed if the order already has a payment.
func (l *MemoryLedger) ProcessPayment(orderID string, _ float64) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.records[orderID]; exists {
		return "", ErrAlreadyProcessed
	}
	paymentID := uuid.New().String()
	l.records[orderID] = record{paymentID: paymentID}
	return paymentID, nil
}

// RefundPayment issues a refund for the given payment. Returns ErrAlreadyRefunded if
// already refunded (idempotency — FR-13).
func (l *MemoryLedger) RefundPayment(orderID, _ string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, exists := l.records[orderID]
	if !exists {
		return "", ErrNotFound
	}
	if r.refundID != "" {
		// Already refunded — return the existing refund ID (idempotent).
		return r.refundID, ErrAlreadyRefunded
	}
	r.refundID = uuid.New().String()
	l.records[orderID] = r
	return r.refundID, nil
}
