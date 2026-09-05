// Package messaging — W3C traceparent inject/extract for kafka-go message headers (R-2).
package messaging

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const traceparentHeader = "traceparent"

// headerCarrier adapts a kafka-go []Header slice to the OTel TextMapCarrier interface.
type headerCarrier struct {
	headers *[]kafkago.Header
}

func (c headerCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c headerCarrier) Set(key, value string) {
	// Replace existing or append.
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, kafkago.Header{Key: key, Value: []byte(value)})
}

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}

// InjectTrace injects the trace context from ctx into the Kafka message headers.
func InjectTrace(ctx context.Context, headers *[]kafkago.Header) {
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier{headers: headers})
}

// ExtractTrace extracts trace context from the Kafka message headers and
// returns a child context with the remote span context attached.
func ExtractTrace(ctx context.Context, headers []kafkago.Header) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, headerCarrier{headers: &headers})
}

// propagationKey is used for the TextMapCarrier.
var _ propagation.TextMapCarrier = headerCarrier{}
