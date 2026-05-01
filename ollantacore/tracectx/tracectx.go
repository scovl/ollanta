package tracectx

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
)

var propagator propagation.TextMapPropagator = propagation.TraceContext{}

// Inject serializes the current trace context into W3C propagation headers.
func Inject(ctx context.Context) (string, string) {
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	return carrier.Get("traceparent"), carrier.Get("tracestate")
}

// Extract restores a context from serialized W3C propagation headers.
func Extract(ctx context.Context, traceParent, traceState string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceParent == "" && traceState == "" {
		return ctx
	}

	carrier := propagation.MapCarrier{}
	if traceParent != "" {
		carrier.Set("traceparent", traceParent)
	}
	if traceState != "" {
		carrier.Set("tracestate", traceState)
	}
	return propagator.Extract(ctx, carrier)
}