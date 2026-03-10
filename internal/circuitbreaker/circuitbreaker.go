// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package circuitbreaker

import (
	"errors"
	"sync"
	"time"

	"github.com/prady-lab/sgh-cli/pkg/logger"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateHalfOpen:
		return "HALF_OPEN"
	case StateOpen:
		return "OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config contains circuit breaker configuration
type Config struct {
	MaxFailures   int           // Number of failures before opening circuit
	Timeout       time.Duration // How long to wait before attempting to close circuit
	MaxRequests   int           // Maximum requests allowed in half-open state
	OnStateChange func(State)   // Callback when state changes
}

// DefaultConfig returns sensible defaults for GitHub API
func DefaultConfig() *Config {
	return &Config{
		MaxFailures: 5,
		Timeout:     30 * time.Second,
		MaxRequests: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu              sync.RWMutex
	config          *Config
	state           State
	failures        int
	requests        int
	lastFailureTime time.Time
}

// New creates a new circuit breaker
func New(config *Config) *CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// Execute runs the given function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return cb.getError()
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// allowRequest determines if a request should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.setState(StateHalfOpen)
			cb.requests = 0
			return true
		}
		return false
	case StateHalfOpen:
		return cb.requests < cb.config.MaxRequests
	default:
		return false
	}
}

// getError returns the appropriate error for the current state
func (cb *CircuitBreaker) getError() error {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		return ErrTooManyRequests
	default:
		return nil
	}
}

// recordResult records the result of a request
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		cb.setState(StateOpen)
	}

	logger.Flog.Warn().
		Str("state", cb.state.String()).
		Int("failures", cb.failures).
		Int("maxFailures", cb.config.MaxFailures).
		Msg("Circuit breaker recorded failure")
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case StateHalfOpen:
		cb.requests++
		if cb.requests >= cb.config.MaxRequests {
			cb.setState(StateClosed)
			cb.failures = 0
		}
	case StateClosed:
		cb.failures = 0
	}

	logger.Flog.Debug().
		Str("state", cb.state.String()).
		Int("requests", cb.requests).
		Int("maxRequests", cb.config.MaxRequests).
		Msg("Circuit breaker recorded success")
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(state State) {
	if cb.state == state {
		return
	}

	oldState := cb.state
	cb.state = state

	logger.Flog.Info().
		Str("oldState", oldState.String()).
		Str("newState", state.String()).
		Msg("Circuit breaker state changed")

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(state)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current statistics
func (cb *CircuitBreaker) GetStats() (State, int, int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state, cb.failures, cb.requests
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(StateClosed)
	cb.failures = 0
	cb.requests = 0
	cb.lastFailureTime = time.Time{}

	logger.Flog.Info().Msg("Circuit breaker reset")
}
