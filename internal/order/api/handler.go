// Package api provides the HTTP handler for the Order Service.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/vladiant/ordersagademo/internal/order/saga"
	"github.com/vladiant/ordersagademo/internal/order/store"
)

const handlerTracer = "ordersagademo/order/api"

// CreateOrderRequest is the JSON body for POST /orders.
type CreateOrderRequest struct {
	OrderID       string  `json:"order_id"`
	Items         []Item  `json:"items"`
	PaymentAmount float64 `json:"payment_amount"`
}

// Item pairs item_id with a quantity.
type Item struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// CreateOrderResponse is returned on 202 Accepted.
type CreateOrderResponse struct {
	OrderID string `json:"order_id"`
	State   string `json:"state"`
}

// Handler handles HTTP requests for order operations.
type Handler struct {
	orchestrator *saga.Orchestrator
}

// NewHandler constructs a Handler.
func NewHandler(orch *saga.Orchestrator) *Handler {
	return &Handler{orchestrator: orch}
}

// ServeHTTP routes requests.
func (h *Handler) ServeHTTP(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.createOrder)
	mux.HandleFunc("GET /healthz", h.healthz)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer(handlerTracer)
	ctx, span := tracer.Start(r.Context(), "POST /orders")
	defer span.End()

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "decode request")
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Assign a server-side order ID if the client did not provide one.
	if req.OrderID == "" {
		req.OrderID = uuid.New().String()
	}

	span.SetAttributes(attribute.String("order.id", req.OrderID))

	items := make([]store.Item, len(req.Items))
	for i, it := range req.Items {
		items[i] = store.Item{ItemID: it.ItemID, Quantity: it.Quantity}
	}
	order := store.Order{
		ID:            req.OrderID,
		Items:         items,
		PaymentAmount: req.PaymentAmount,
	}

	// event_id is generated once here, before any retry (R-7).
	eventID := uuid.New().String()

	if err := h.orchestrator.StartSaga(ctx, order, eventID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "start saga")
		slog.ErrorContext(ctx, "start saga failed", "order_id", req.OrderID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(CreateOrderResponse{
		OrderID: req.OrderID,
		State:   string(store.StateAwaitingPayment),
	}); err != nil {
		slog.ErrorContext(ctx, "encode response", "error", err)
	}
}
