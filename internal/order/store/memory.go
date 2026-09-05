package store

import "sync"

// MemoryStore is a sync.RWMutex-guarded in-memory order store.
type MemoryStore struct {
	mu     sync.RWMutex
	orders map[string]Order
}

// NewMemoryStore returns an initialised MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{orders: make(map[string]Order)}
}

func (s *MemoryStore) Save(order Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[order.ID] = order
	return nil
}

func (s *MemoryStore) Get(orderID string) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[orderID]
	if !ok {
		return Order{}, ErrNotFound
	}
	return o, nil
}

func (s *MemoryStore) UpdateState(orderID string, state OrderState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderID]
	if !ok {
		return ErrNotFound
	}
	o.State = state
	s.orders[orderID] = o
	return nil
}

func (s *MemoryStore) SetPaymentID(orderID, paymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderID]
	if !ok {
		return ErrNotFound
	}
	o.PaymentID = paymentID
	s.orders[orderID] = o
	return nil
}

func (s *MemoryStore) SetReservationID(orderID, reservationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderID]
	if !ok {
		return ErrNotFound
	}
	o.ReservationID = reservationID
	s.orders[orderID] = o
	return nil
}
