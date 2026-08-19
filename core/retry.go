package core

import (
	"context"
	"time"
)

type retryPolicy struct {
	delays      []time.Duration
	shouldRetry func(error) bool
}

var defaultRetryPolicy = retryPolicy{
	delays: []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	},
}

func retryOperationForDB(c context.Context, _ string, fn func() error) error {
	return retryOperationWithPolicy(c, defaultRetryPolicy, fn)
}

func retryOperationWithPolicy(c context.Context, policy retryPolicy, fn func() error) (err error) {
	delays := policy.delays
	if len(delays) == 0 {
		delays = []time.Duration{0}
	}

	for i := 0; i < len(delays); i++ {
		if c != nil {
			if cerr := c.Err(); cerr != nil {
				return cerr
			}
		}

		if err = fn(); err == nil {
			return nil
		}

		if policy.shouldRetry != nil && !policy.shouldRetry(err) {
			return err
		}
		if i == len(delays)-1 {
			return err
		}

		delay := delays[i]
		if delay <= 0 {
			continue
		}

		timer := time.NewTimer(delay)
		if c == nil {
			<-timer.C
			continue
		}

		select {
		case <-timer.C:
		case <-c.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return c.Err()
		}
	}

	return err
}
