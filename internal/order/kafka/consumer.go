// Package kafka provides the Kafka consumer for the Order Service.
package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/vladiant/ordersagademo/internal/messaging"
	"github.com/vladiant/ordersagademo/internal/order/saga"
)

const consumerTracer = "ordersagademo/order/kafka/consumer"

// Consumer reads PaymentProcessed, PaymentFailed, and PaymentRefunded events
// and drives the saga orchestrator. It deduplicates on event_id (R-1).
type Consumer struct {
	readers      []*kafkago.Reader
	orchestrator *saga.Orchestrator
	seen         map[string]struct{} // event_id deduplication
	counter      metric.Int64Counter
}

// NewConsumer creates readers for all three inbound topics.
func NewConsumer(brokers []string, groupID string, orch *saga.Orchestrator) (*Consumer, error) {
	topics := []string{
		"saga.payments.processed",
		"saga.payments.failed",
		"saga.payments.refunded",
	}
	readers := make([]*kafkago.Reader, len(topics))
	for i, t := range topics {
		readers[i] = kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: brokers,
			Topic:   t,
			GroupID: groupID,
		})
	}

	meter := otel.GetMeterProvider().Meter(consumerTracer)
	counter, err := meter.Int64Counter("kafka_messages_consumed_total",
		metric.WithDescription("Kafka messages consumed per topic and consumer group"))
	if err != nil {
		return nil, err
	}

	return &Consumer{
		readers:      readers,
		orchestrator: orch,
		seen:         make(map[string]struct{}),
		counter:      counter,
	}, nil
}

// Run starts all reader goroutines. It blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) {
	for _, r := range c.readers {
		go c.consumeLoop(ctx, r)
	}
	<-ctx.Done()
}

func (c *Consumer) consumeLoop(ctx context.Context, r *kafkago.Reader) {
	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.ErrorContext(ctx, "order consumer read error", "error", err)
			continue
		}

		// Extract trace context from Kafka headers (R-2).
		msgCtx := messaging.ExtractTrace(ctx, msg.Headers)

		c.counter.Add(msgCtx, 1, metric.WithAttributes(
			attribute.String("topic", r.Config().Topic),
			attribute.String("consumer_group", r.Config().GroupID),
		))

		c.handleMessage(msgCtx, msg)
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg kafkago.Message) {
	var env messaging.Envelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		slog.ErrorContext(ctx, "order consumer unmarshal envelope", "error", err)
		return
	}

	// Idempotency: discard duplicates (R-1).
	if _, seen := c.seen[env.EventID]; seen {
		slog.WarnContext(ctx, "duplicate event discarded", "event_id", env.EventID, "event_type", env.EventType)
		return
	}
	c.seen[env.EventID] = struct{}{}

	switch env.EventType {
	case "PaymentProcessed":
		var evt messaging.PaymentProcessedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.ErrorContext(ctx, "unmarshal PaymentProcessed", "error", err)
			return
		}
		if err := c.orchestrator.OnPaymentProcessed(ctx, evt.OrderID, evt.PaymentID); err != nil {
			slog.ErrorContext(ctx, "OnPaymentProcessed", "order_id", evt.OrderID, "error", err)
		}
	case "PaymentFailed":
		var evt messaging.PaymentFailedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.ErrorContext(ctx, "unmarshal PaymentFailed", "error", err)
			return
		}
		if err := c.orchestrator.OnPaymentFailed(ctx, evt.OrderID, evt.Reason); err != nil {
			slog.ErrorContext(ctx, "OnPaymentFailed", "order_id", evt.OrderID, "error", err)
		}
	case "PaymentRefunded":
		var evt messaging.PaymentRefundedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.ErrorContext(ctx, "unmarshal PaymentRefunded", "error", err)
			return
		}
		if err := c.orchestrator.OnPaymentRefunded(ctx, evt.OrderID); err != nil {
			slog.ErrorContext(ctx, "OnPaymentRefunded", "order_id", evt.OrderID, "error", err)
		}
	default:
		slog.WarnContext(ctx, "order consumer: unknown event type", "event_type", env.EventType)
	}
}

// Close closes all readers.
func (c *Consumer) Close() error {
	for _, r := range c.readers {
		_ = r.Close()
	}
	return nil
}
