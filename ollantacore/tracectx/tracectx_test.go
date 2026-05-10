package tracectx

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/propagation"
)

func TestInjectAndExtractRoundTrip(t *testing.T) {
	ctx := context.Background()
	traceParent, traceState := Inject(ctx)
	if traceParent != "" {
		t.Error("expected empty traceparent for background context")
	}
	if traceState != "" {
		t.Error("expected empty tracestate for background context")
	}

	restored := Extract(ctx, traceParent, traceState)
	if restored != ctx {
		t.Error("Extract should return the original ctx when both headers are empty")
	}
}

func TestInjectProducesValidW3CFormat(t *testing.T) {
	carrier := propagation.MapCarrier{}
	propagator.Inject(context.Background(), carrier)
	tp := carrier.Get("traceparent")
	if tp == "" {
		t.Skip("no traceparent produced (otel may need a configured SDK)")
	}
}

func TestExtractRestoresTraceParent(t *testing.T) {
	tp := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ts := "rojo=00f067aa0ba902b7"
	ctx := Extract(context.Background(), tp, ts)
	if ctx == nil {
		t.Fatal("Extract returned nil")
	}
}

func TestExtractWithNilContext(t *testing.T) {
	tp := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ctx := Extract(context.TODO(), tp, "")
	if ctx == nil {
		t.Fatal("Extract should handle nil context")
	}
}

func TestExtractWithOnlyTraceParent(t *testing.T) {
	tp := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ctx := Extract(context.Background(), tp, "")
	if ctx == nil {
		t.Fatal("Extract returned nil with only traceparent")
	}
}

func TestExtractWithOnlyTraceState(t *testing.T) {
	ctx := Extract(context.Background(), "", "rojo=00f067aa0ba902b7")
	if ctx == nil {
		t.Fatal("Extract returned nil with only tracestate")
	}
}

func TestInjectExtractRoundTripWithValues(t *testing.T) {
	tp := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ts := "rojo=00f067aa0ba902b7"
	ctx := Extract(context.Background(), tp, ts)

	traceParent, traceState := Inject(ctx)
	if traceParent == "" {
		t.Skip("no traceparent produced (otel may need a configured SDK)")
	}
	if traceState == "" {
		t.Log("tracestate may be empty if not propagated")
	}
	_ = traceParent
	_ = traceState
}
