// Package queue retry with exponential backoff and randomized full jitter.
// 100% pure 7-bit ASCII
package queue

import (
	"crypto/rand"
	"math/big"
	"time"
)

// RetryPolicy defines backoff and ceiling bounds for failed message handling.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy provides standard exponential backoff with jitter parameters.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 5,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
	}
}

// ComputeDelay calculates exponential delay with full randomized jitter to prevent thundering herds.
// Formula: uniform_random(0, min(MaxDelay, BaseDelay * 2^attempt))
func (p RetryPolicy) ComputeDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	shift := uint(min(attempt, 16))
	multiplier := 1 << shift
	calculated := p.BaseDelay * time.Duration(multiplier)
	ceiling := min(calculated, p.MaxDelay)

	if ceiling <= 0 {
		return 0
	}

	n, err := rand.Int(rand.Reader, big.NewInt(ceiling.Nanoseconds()))
	if err != nil {
		return ceiling / 2 // Fallback if crypto rand fails
	}

	return time.Duration(n.Int64())
}
