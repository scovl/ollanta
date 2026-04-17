package breaker_test

import (
	"errors"
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantaweb/breaker"
)

var errFake = errors.New("fake error")

func TestBreakerClosedStateAllowsRequests(t *testing.T) {
	t.Parallel()
	b := breaker.New(3, time.Second, 1)
	for i := 0; i < 5; i++ {
		if err := b.Do(func() error { return nil }); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	}
}

func TestBreakerOpensAfterMaxFailures(t *testing.T) {
	t.Parallel()
	b := breaker.New(3, 30*time.Second, 1)

	for i := 0; i < 3; i++ {
		_ = b.Do(func() error { return errFake })
	}

	// Next call should be rejected by open circuit
	err := b.Do(func() error { return nil })
	if !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("expected ErrOpen after max failures, got %v", err)
	}
	if b.State() != "open" {
		t.Errorf("expected state 'open', got %q", b.State())
	}
}

func TestBreakerTransitionsToHalfOpen(t *testing.T) {
	t.Parallel()
	b := breaker.New(2, 10*time.Millisecond, 1)

	_ = b.Do(func() error { return errFake })
	_ = b.Do(func() error { return errFake })

	// Wait for reset timeout
	time.Sleep(20 * time.Millisecond)

	// Should allow one probe request (half-open)
	if err := b.Allow(); err != nil {
		t.Fatalf("expected half-open probe to be allowed, got %v", err)
	}
	if b.State() != "half_open" {
		t.Errorf("expected state 'half_open', got %q", b.State())
	}
}

func TestBreakerClosesOnSuccessAfterHalfOpen(t *testing.T) {
	t.Parallel()
	b := breaker.New(1, 10*time.Millisecond, 1)

	_ = b.Do(func() error { return errFake })
	time.Sleep(20 * time.Millisecond)

	// Successful probe should close the breaker
	if err := b.Do(func() error { return nil }); err != nil {
		t.Fatalf("expected success in half-open, got %v", err)
	}
	if b.State() != "closed" {
		t.Errorf("expected state 'closed' after successful probe, got %q", b.State())
	}
}
