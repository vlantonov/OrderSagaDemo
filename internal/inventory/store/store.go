// Package store defines the StockStore interface and in-memory implementation.
package store

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// ErrInsufficientStock is returned when requested items cannot all be reserved.
var ErrInsufficientStock = errors.New("insufficient stock")

// ErrReservationNotFound is returned when a reservation ID is unknown.
var ErrReservationNotFound = errors.New("reservation not found")

// Store is the read/write interface for stock management.
type Store interface {
	ReserveStock(orderID string, items []Item) (reservationID string, err error)
	ReleaseStock(orderID, reservationID string) error
}

// Item pairs an item ID with a quantity.
type Item struct {
	ItemID   string
	Quantity int
}

// stockEntry tracks current stock and any active reservation.
type stockEntry struct {
	total    int
	reserved int
}

// reservationRecord maps a reservation ID to the items it covers.
type reservationRecord struct {
	orderID string
	items   []Item
}

// MemoryStore is a thread-safe in-memory stock store.
type MemoryStore struct {
	mu           sync.Mutex
	stock        map[string]*stockEntry       // item_id → stock
	reservations map[string]reservationRecord // reservation_id → record
}

// NewMemoryStore pre-populates stock with generous quantities for the demo.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		stock:        make(map[string]*stockEntry),
		reservations: make(map[string]reservationRecord),
	}
	// Pre-seed items with ample stock so the happy path always succeeds.
	for _, id := range []string{"item-A", "item-B", "item-C", "item-D", "widget-01"} {
		s.stock[id] = &stockEntry{total: 1000}
	}
	return s
}

// ReserveStock atomically marks units as reserved and returns a reservationID.
// Any item_id not present in the store is treated as out-of-stock.
func (s *MemoryStore) ReserveStock(orderID string, items []Item) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate all items first (all-or-nothing).
	for _, it := range items {
		entry, ok := s.stock[it.ItemID]
		if !ok || entry.total-entry.reserved < it.Quantity {
			return "", ErrInsufficientStock
		}
	}

	// Apply reservations.
	for _, it := range items {
		s.stock[it.ItemID].reserved += it.Quantity
	}
	reservationID := uuid.New().String()
	s.reservations[reservationID] = reservationRecord{orderID: orderID, items: items}
	return reservationID, nil
}

// ReleaseStock reverses a prior reservation.
func (s *MemoryStore) ReleaseStock(_, reservationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.reservations[reservationID]
	if !ok {
		return ErrReservationNotFound
	}
	for _, it := range rec.items {
		if entry, exists := s.stock[it.ItemID]; exists {
			entry.reserved -= it.Quantity
			if entry.reserved < 0 {
				entry.reserved = 0
			}
		}
	}
	delete(s.reservations, reservationID)
	return nil
}
