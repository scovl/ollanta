// Package breaker implements a lightweight circuit breaker (Closed → Open → Half-Open)
// using only Go stdlib atomic operations. No external dependencies.
package breaker

import (
	"errors"
	"sync/atomic"
	"time"
)

// ErrOpen is returned when the circuit is open and the call is rejected.
var ErrOpen = errors.New("circuit breaker open")

// state constants stored atomically.
const (
	stateClosed   int32 = 0
	stateOpen     int32 = 1
	stateHalfOpen int32 = 2
)

// Breaker is a simple circuit breaker with half-open recovery.
// Safe for concurrent use.
type Breaker struct {
	maxFailures  int32
	resetTimeout time.Duration
	halfOpenMax  int32

	state        atomic.Int32
	failures     atomic.Int32
	halfOpenUsed atomic.Int32
	openedAt     atomic.Int64 // unix nano
}

// New creates a Breaker with the given parameters.
//
//	maxFailures  — consecutive failures to open the circuit.
//	resetTimeout — how long the circuit stays open before half-opening.
//	halfOpenMax  — max concurrent test requests in half-open state.
func New(maxFailures int, resetTimeout time.Duration, halfOpenMax int) *Breaker {
	b := &Breaker{
		maxFailures:  int32(maxFailures),
		resetTimeout: resetTimeout,
		halfOpenMax:  int32(halfOpenMax),
	}
	b.state.Store(stateClosed)
	return b
}

// Allow returns nil if the call should proceed, or ErrOpen if rejected.
// In half-open state only halfOpenMax concurrent requests are allowed.
func (b *Breaker) Allow() error {
	switch b.state.Load() {
	case stateClosed:
		return nil

	case stateOpen:
		openedAt := time.Unix(0, b.openedAt.Load())
		if time.Since(openedAt) < b.resetTimeout {
			return ErrOpen
		}
		// Transition to half-open.
		if b.state.CompareAndSwap(stateOpen, stateHalfOpen) {
			b.halfOpenUsed.Store(0)
		}
		fallthrough

	case stateHalfOpen:
		used := b.halfOpenUsed.Add(1)
		if used > b.halfOpenMax {
			b.halfOpenUsed.Add(-1)
			return ErrOpen
		}
		return nil
	}
	return nil
}

// Success records a successful call. Closes the circuit if in half-open state.
func (b *Breaker) Success() {
	switch b.state.Load() {
	case stateHalfOpen:
		b.failures.Store(0)
		b.halfOpenUsed.Store(0)
		b.state.Store(stateClosed)
	case stateClosed:
		b.failures.Store(0)
	}
}

// Failure records a failed call. Opens the circuit when maxFailures is reached.
func (b *Breaker) Failure() {
	switch b.state.Load() {
	case stateHalfOpen:
		// Test request failed — go back to open.
		b.halfOpenUsed.Add(-1)
		b.openedAt.Store(time.Now().UnixNano())
		b.state.Store(stateOpen)
	case stateClosed:
		f := b.failures.Add(1)
		if f >= b.maxFailures {
			b.openedAt.Store(time.Now().UnixNano())
			b.state.Store(stateOpen)
		}
	}
}

// State returns a human-readable state name (for metrics/health).
func (b *Breaker) State() string {
	switch b.state.Load() {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

// Do wraps fn with circuit breaker protection.
// Returns ErrOpen immediately if the circuit is open.
// Calls Success or Failure automatically based on the fn error.
func (b *Breaker) Do(fn func() error) error {
	if err := b.Allow(); err != nil {
		return err
	}
	err := fn()
	if err != nil {
		b.Failure()
	} else {
		b.Success()
	}
	return err
}
