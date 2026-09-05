package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vladiant/ordersagademo/internal/payment/ledger"
)

// TestProcessPayment_Idempotent verifies that processing the same order twice
// returns ErrAlreadyProcessed on the second call (FR-13 idempotency layer).
func TestProcessPayment_Idempotent(t *testing.T) {
	l := ledger.NewMemoryLedger()

	id1, err := l.ProcessPayment("order-A", 42.00)
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	id2, err := l.ProcessPayment("order-A", 42.00)
	assert.ErrorIs(t, err, ledger.ErrAlreadyProcessed)
	assert.Empty(t, id2)
}

// TestRefundPayment_Idempotent verifies that a second refund call for the same
// order returns ErrAlreadyRefunded and the same refundID (FR-13).
func TestRefundPayment_Idempotent(t *testing.T) {
	l := ledger.NewMemoryLedger()

	payID, err := l.ProcessPayment("order-B", 10.00)
	require.NoError(t, err)

	refund1, err := l.RefundPayment("order-B", payID)
	require.NoError(t, err)
	assert.NotEmpty(t, refund1)

	// Second refund must return ErrAlreadyRefunded and the same refundID.
	refund2, err := l.RefundPayment("order-B", payID)
	assert.ErrorIs(t, err, ledger.ErrAlreadyRefunded)
	assert.Equal(t, refund1, refund2, "idempotent refund must return same refundID")
}

// TestRefundPayment_NotFound verifies that refunding a non-existent order returns ErrNotFound.
func TestRefundPayment_NotFound(t *testing.T) {
	l := ledger.NewMemoryLedger()
	_, err := l.RefundPayment("no-such-order", "pay-xyz")
	assert.ErrorIs(t, err, ledger.ErrNotFound)
}
