// Package telemetry keeps compatibility imports for legacy ollantaweb code.
package telemetry

import (
	"net/http"

	base "github.com/scovl/ollanta/adapter/secondary/telemetry"
)

type Counter = base.Counter
type Gauge = base.Gauge
type Histogram = base.Histogram
type Registry = base.Registry
type Metrics = base.Metrics

var NewRegistry = base.NewRegistry
var NewMetrics = base.NewMetrics
var TraceID = base.TraceID
var SetupLogger = base.SetupLogger
var WithTraceAttrs = base.WithTraceAttrs

func TraceIDMiddleware(next http.Handler) http.Handler {
	return base.TraceIDMiddleware(next)
}
