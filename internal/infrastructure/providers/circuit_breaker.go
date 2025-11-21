package providers

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half-open"
)

type CircuitBreaker struct {
	name             string
	failureThreshold int
	resetTimeout     time.Duration
	logger           *zap.Logger

	state           State
	failures        int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

func NewCircuitBreaker(name string, threshold int, timeout time.Duration, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		failureThreshold: threshold,
		resetTimeout:     timeout,
		logger:           logger,
		state:            StateClosed,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allow() {
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}

	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == StateClosed {
		return true
	}

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			// Move to half-open to test
			cb.mu.RUnlock()
			cb.mu.Lock()
			if cb.state == StateOpen {
				cb.logger.Info("Circuit breaker moving to half-open", zap.String("name", cb.name))
				cb.state = StateHalfOpen
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	}

	// Half-open: allow one request to test
	return true
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.state == StateClosed && cb.failures >= cb.failureThreshold {
		cb.logger.Warn("Circuit breaker moving to open", zap.String("name", cb.name), zap.Int("failures", cb.failures))
		cb.state = StateOpen
	} else if cb.state == StateHalfOpen {
		cb.logger.Warn("Circuit breaker moving back to open from half-open", zap.String("name", cb.name))
		cb.state = StateOpen
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen || cb.failures > 0 {
		cb.logger.Info("Circuit breaker reset to closed", zap.String("name", cb.name))
		cb.state = StateClosed
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
