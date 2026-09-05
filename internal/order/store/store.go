// Package store defines the OrderStore interface and the in-memory implementation.
package store

import "errors"

// ErrNotFound is returned when an order does not exist.
var ErrNotFound = errors.New("order not found")

// OrderState represents the saga state machine position for an order.
type OrderState string

const (
	StatePending         OrderState = "PENDING"
	StateAwaitingPayment OrderState = "AWAITING_PAYMENT"
	StateReserving       OrderState = "RESERVING"
	StateCompleted       OrderState = "COMPLETED"
	StateFailed          OrderState = "FAILED"
	StateCompensating    OrderState = "COMPENSATING"
	StateCompensated     OrderState = "COMPENSATED"
)

// Order holds per-order saga data.
type Order struct {
	ID            string
	State         OrderState
	Items         []Item
	PaymentAmount float64
	PaymentID     string // populated after payment step
	ReservationID string // populated after inventory step
}

// Item pairs an item ID with a quantity.
type Item struct {
	ItemID   string
	Quantity int
}

// Store is the read/write interface for order persistence.
type Store interface {
	Save(order Order) error
	Get(orderID string) (Order, error)
	UpdateState(orderID string, state OrderState) error
	SetPaymentID(orderID, paymentID string) error
	SetReservationID(orderID, reservationID string) error
}
