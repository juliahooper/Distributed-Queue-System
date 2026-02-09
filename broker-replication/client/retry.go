package client

import (
	"context"
	"time"
)

// RetryPolicy controls how the producer retries failed operations.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	MaxAttempts int
	// BaseBackoff is the initial backoff duration; backoff grows exponentially.
	BaseBackoff time.Duration
	// MaxBackoff is an optional cap on backoff; zero means no cap.
	MaxBackoff time.Duration
}

// DefaultRetryPolicy returns a conservative default retry policy suitable
// for local development.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseBackoff: 50 * time.Millisecond,
		MaxBackoff:  500 * time.Millisecond,
	}
}

// withRetry executes fn according to the given policy, respecting ctx
// cancellation. It returns on the first success or the last error.
func withRetry(ctx context.Context, policy RetryPolicy, fn func() error) error {
	if policy.MaxAttempts <= 0 {
		// Treat non-positive as a single attempt with no backoff.
		policy.MaxAttempts = 1
	}
	if policy.BaseBackoff <= 0 {
		policy.BaseBackoff = 10 * time.Millisecond
	}

	var attempt int
	var backoff = policy.BaseBackoff

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		attempt++
		if attempt >= policy.MaxAttempts {
			return err
		}

		sleep := backoff
		if policy.MaxBackoff > 0 && sleep > policy.MaxBackoff {
			sleep = policy.MaxBackoff
		}

		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		backoff *= 2
	}
}

