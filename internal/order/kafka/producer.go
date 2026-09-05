// Package kafka provides the Kafka producer for the Order Service.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/vladiant/ordersagademo/internal/messaging"
)

const producerTracer = "ordersagademo/order/kafka/producer"

// Producer wraps a kafka-go writer and injects trace context into message headers.
type Producer struct {
	writer  *kafkago.Writer
	counter metric.Int64Counter
}

// NewProducer creates a Producer targeting the given broker.
func NewProducer(brokers []string) (*Producer, error) {
	w := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Balancer:     &kafkago.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafkago.RequireOne,
		Async:        false,
	}

	meter := otel.GetMeterProvider().Meter(producerTracer)
	counter, err := meter.Int64Counter("kafka_messages_produced_total",
		metric.WithDescription("Kafka messages produced per topic"))
	if err != nil {
		return nil, fmt.Errorf("producer metric: %w", err)
	}
	return &Producer{writer: w, counter: counter}, nil
}

// Publish marshals payload to JSON, injects the trace context, and writes to topic.
func (p *Producer) Publish(ctx context.Context, topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	headers := make([]kafkago.Header, 0, 2)
	messaging.InjectTrace(ctx, &headers)

	msg := kafkago.Message{
		Topic:   topic,
		Value:   data,
		Headers: headers,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write to %s: %w", topic, err)
	}

	p.counter.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic)))
	slog.InfoContext(ctx, "event published", "topic", topic)
	return nil
}

// Close shuts down the underlying writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}
