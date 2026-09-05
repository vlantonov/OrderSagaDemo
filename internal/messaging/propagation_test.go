package messaging_test

import (
	"context"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/vladiant/ordersagademo/internal/messaging"
)

func init() {
	// Install a no-op SDK so trace context round-trips work in tests.
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	))
}

// TestInjectExtractRoundtrip verifies that a trace context injected into Kafka headers
// can be extracted back and produces the same traceID.
func TestInjectExtractRoundtrip(t *testing.T) {
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	headers := make([]kafkago.Header, 0)
	messaging.InjectTrace(ctx, &headers)

	assert.NotEmpty(t, headers, "expected at least one header")
	var found bool
	for _, h := range headers {
		if h.Key == "traceparent" {
			found = true
			assert.NotEmpty(t, h.Value)
		}
	}
	assert.True(t, found, "traceparent header must be present")

	extracted := messaging.ExtractTrace(context.Background(), headers)
	extractedSpan := sdktrace.NewTracerProvider().Tracer("t")
	_, childSpan := otel.Tracer("t").Start(extracted, "child")
	_ = extractedSpan
	defer childSpan.End()

	// The extracted context should carry the same trace ID.
	origTraceID := span.SpanContext().TraceID().String()
	childTraceID := childSpan.SpanContext().TraceID().String()
	assert.Equal(t, origTraceID, childTraceID, "trace IDs must match after extract")
}

// TestEnvelopeNewEnvelope checks that NewEnvelope populates required fields.
func TestEnvelopeNewEnvelope(t *testing.T) {
	env := messaging.NewEnvelope("OrderCreated", "evt-xyz")
	assert.Equal(t, "OrderCreated", env.EventType)
	assert.Equal(t, "evt-xyz", env.EventID)
	assert.NotEmpty(t, env.Timestamp)
}
