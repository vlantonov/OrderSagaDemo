// Package kafka provides the Kafka consumer and producer for the Payment Service.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/vladiant/ordersagademo/internal/messaging"
	"github.com/vladiant/ordersagademo/internal/payment/ledger"
)

const (
	serviceName    = "ordersagademo/payment/kafka"
	tracerName     = serviceName
)

// Worker combines the Kafka consumer (OrderCreated, CompensatePayment) and
// the producer (PaymentProcessed, PaymentFailed, PaymentRefunded).
type Worker struct {
	readers  []*kafkago.Reader
	writer   *kafkago.Writer
	ledger   ledger.Ledger
	seen     map[string]struct{} // event_id deduplication (R-1)
	tracer   trace.Tracer
	produced metric.Int64Counter
	consumed metric.Int64Counter
}

// NewWorker creates a Worker targeting the given broker and consumer group.
func NewWorker(brokers []string, groupID string, l ledger.Ledger) (*Worker, error) {
	topics := []string{"saga.orders.created", "saga.payments.compensate"}
	readers := make([]*kafkago.Reader, len(topics))
	for i, t := range topics {
		readers[i] = kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: brokers,
			Topic:   t,
			GroupID: groupID,
		})
	}

	w := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafkago.RequireOne,
	}

	meter := otel.GetMeterProvider().Meter(serviceName)
	produced, err := meter.Int64Counter("kafka_messages_produced_total",
		metric.WithDescription("Kafka messages produced per topic"))
	if err != nil {
		return nil, fmt.Errorf("payment produced metric: %w", err)
	}
	consumed, err := meter.Int64Counter("kafka_messages_consumed_total",
		metric.WithDescription("Kafka messages consumed per topic and consumer group"))
	if err != nil {
		return nil, fmt.Errorf("payment consumed metric: %w", err)
	}

	return &Worker{
		readers:  readers,
		writer:   w,
		ledger:   l,
		seen:     make(map[string]struct{}),
		tracer:   otel.Tracer(tracerName),
		produced: produced,
		consumed: consumed,
	}, nil
}

// Run starts consumer goroutines. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	for _, r := range w.readers {
		go w.consumeLoop(ctx, r)
	}
	<-ctx.Done()
}

func (w *Worker) consumeLoop(ctx context.Context, r *kafkago.Reader) {
	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.ErrorContext(ctx, "payment consumer read error", "error", err)
			continue
		}

		msgCtx := messaging.ExtractTrace(ctx, msg.Headers)
		w.consumed.Add(msgCtx, 1, metric.WithAttributes(
			attribute.String("topic", r.Config().Topic),
			attribute.String("consumer_group", r.Config().GroupID),
		))

		w.handleMessage(msgCtx, msg)
	}
}

func (w *Worker) handleMessage(ctx context.Context, msg kafkago.Message) {
	var env messaging.Envelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		slog.ErrorContext(ctx, "payment unmarshal envelope", "error", err)
		return
	}

	// Idempotency (R-1).
	if _, seen := w.seen[env.EventID]; seen {
		slog.WarnContext(ctx, "duplicate event discarded", "event_id", env.EventID, "event_type", env.EventType)
		return
	}
	w.seen[env.EventID] = struct{}{}

	switch env.EventType {
	case "OrderCreated":
		var evt messaging.OrderCreatedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.ErrorContext(ctx, "unmarshal OrderCreated", "error", err)
			return
		}
		w.processPayment(ctx, evt)
	case "CompensatePayment":
		var evt messaging.CompensatePaymentEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.ErrorContext(ctx, "unmarshal CompensatePayment", "error", err)
			return
		}
		w.refundPayment(ctx, evt)
	default:
		slog.WarnContext(ctx, "payment consumer: unknown event type", "event_type", env.EventType)
	}
}

func (w *Worker) processPayment(ctx context.Context, evt messaging.OrderCreatedEvent) {
	ctx, span := w.tracer.Start(ctx, "payment.processPayment",
		trace.WithAttributes(attribute.String("order.id", evt.OrderID)))
	defer span.End()

	paymentID, err := w.ledger.ProcessPayment(evt.OrderID, evt.PaymentAmount)
	if err != nil {
		slog.WarnContext(ctx, "process payment failed", "order_id", evt.OrderID, "error", err)
		// Publish PaymentFailed — event_id generated once (R-7).
		outEvt := messaging.PaymentFailedEvent{
			Envelope: messaging.NewEnvelope("PaymentFailed", uuid.New().String()),
			OrderID:  evt.OrderID,
			Reason:   err.Error(),
		}
		w.publish(ctx, "saga.payments.failed", outEvt)
		return
	}

	slog.InfoContext(ctx, "payment processed", "order_id", evt.OrderID, "payment_id", paymentID)
	outEvt := messaging.PaymentProcessedEvent{
		Envelope:  messaging.NewEnvelope("PaymentProcessed", uuid.New().String()),
		OrderID:   evt.OrderID,
		PaymentID: paymentID,
	}
	w.publish(ctx, "saga.payments.processed", outEvt)
}

func (w *Worker) refundPayment(ctx context.Context, evt messaging.CompensatePaymentEvent) {
	ctx, span := w.tracer.Start(ctx, "payment.refundPayment",
		trace.WithAttributes(attribute.String("order.id", evt.OrderID)))
	defer span.End()

	refundID, err := w.ledger.RefundPayment(evt.OrderID, evt.PaymentID)
	if err == ledger.ErrAlreadyRefunded {
		// Idempotent — already refunded, publish refunded event with existing ID.
		slog.WarnContext(ctx, "payment already refunded — re-publishing", "order_id", evt.OrderID)
	} else if err != nil {
		slog.ErrorContext(ctx, "refund payment failed", "order_id", evt.OrderID, "error", err)
		return
	}

	slog.InfoContext(ctx, "payment refunded", "order_id", evt.OrderID, "refund_id", refundID)
	outEvt := messaging.PaymentRefundedEvent{
		Envelope:  messaging.NewEnvelope("PaymentRefunded", uuid.New().String()),
		OrderID:   evt.OrderID,
		PaymentID: evt.PaymentID,
		RefundID:  refundID,
	}
	w.publish(ctx, "saga.payments.refunded", outEvt)
}

func (w *Worker) publish(ctx context.Context, topic string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "marshal payment event", "error", err)
		return
	}

	headers := make([]kafkago.Header, 0, 2)
	messaging.InjectTrace(ctx, &headers)

	if err := w.writer.WriteMessages(ctx, kafkago.Message{
		Topic:   topic,
		Value:   data,
		Headers: headers,
	}); err != nil {
		slog.ErrorContext(ctx, "write payment event", "topic", topic, "error", err)
		return
	}

	w.produced.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic)))
}

// Close shuts down all readers and the writer.
func (w *Worker) Close() {
	for _, r := range w.readers {
		_ = r.Close()
	}
	_ = w.writer.Close()
}
