// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerBasicFlow(t *testing.T) {
	config := &Config{
		MaxFailures: 3,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 2,
	}
	cb := New(config)

	// Initially should be closed
	if cb.GetState() != StateClosed {
		t.Errorf("expected initial state to be CLOSED, got %v", cb.GetState())
	}

	// Test successful execution
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestCircuitBreakerStateTransitions(t *testing.T) {
	config := &Config{
		MaxFailures: 2,
		Timeout:     200 * time.Millisecond,
		MaxRequests: 1,
	}
	cb := New(config)

	// Force failures to open circuit
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return errors.New("fail") })
	}

	// Should be open now
	if cb.GetState() != StateOpen {
		t.Errorf("expected state to be OPEN, got %v", cb.GetState())
	}

	// Should reject requests while open
	err := cb.Execute(func() error { return nil })
	if err == nil {
		t.Error("expected error from open circuit, got nil")
	}

	// Wait for timeout and test half-open
	time.Sleep(250 * time.Millisecond)

	// Should allow one request in half-open
	err = cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("expected nil error in half-open, got %v", err)
	}

	// Should be closed again after success
	if cb.GetState() != StateClosed {
		t.Errorf("expected state to be CLOSED after success, got %v", cb.GetState())
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	config := &Config{
		MaxFailures: 10,
		Timeout:     100 * time.Millisecond,
		MaxRequests: 5,
	}
	cb := New(config)

	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	// Run concurrent requests
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := cb.Execute(func() error {
				time.Sleep(10 * time.Millisecond) // Simulate work
				if id%3 == 0 {
					return errors.New("simulated error")
				}
				return nil
			})

			mu.Lock()
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify some operations completed
	if successCount == 0 {
		t.Error("expected some successful operations")
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	config := &Config{
		MaxFailures: 1,
		Timeout:     50 * time.Millisecond,
		MaxRequests: 1,
	}
	cb := New(config)

	// Force open
	cb.Execute(func() error { return errors.New("fail") })

	if cb.GetState() != StateOpen {
		t.Errorf("expected OPEN state, got %v", cb.GetState())
	}

	// Reset should close circuit
	cb.Reset()

	if cb.GetState() != StateClosed {
		t.Errorf("expected CLOSED state after reset, got %v", cb.GetState())
	}
}
