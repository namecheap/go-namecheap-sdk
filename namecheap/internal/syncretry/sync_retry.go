// Package syncretry provides a mutex-guarded retry mechanism with configurable
// delays between attempts.
package syncretry

import (
	"errors"
	"sync"
	"time"
)

// ErrRetry signals that the operation should be retried.
var ErrRetry = errors.New("retry error")

// ErrRetryAttempts is returned when all retry attempts have been exhausted
// without a successful result.
var ErrRetryAttempts = errors.New("retry attempts error")

// Options holds configuration for a SyncRetry instance.
type Options struct {
	// Delays specifies the wait durations in seconds between successive retry
	// attempts. The number of retries equals the length of this slice.
	Delays []int
}

// SyncRetry executes a function with mutex-guarded retries on transient
// failures signalled by ErrRetry.
type SyncRetry struct {
	m       *sync.Mutex
	options *Options
}

// NewSyncRetry returns a new SyncRetry configured with the given options.
func NewSyncRetry(options *Options) *SyncRetry {
	return &SyncRetry{
		m:       &sync.Mutex{},
		options: options,
	}
}

// Do calls f and, if f returns ErrRetry, acquires the mutex and retries f
// after each delay configured in Options.Delays. It returns nil on the first
// successful call, any non-retriable error immediately, or ErrRetryAttempts
// when all retries are exhausted.
func (sq *SyncRetry) Do(f func() error) error {
	err := f()
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrRetry) {
		return err
	}

	sq.m.Lock()
	defer sq.m.Unlock()

	for _, delay := range sq.options.Delays {
		time.Sleep(time.Duration(delay) * time.Second)
		err = f()
		if err == nil {
			return nil
		}

		if errors.Is(err, ErrRetry) {
			continue
		}
		return err
	}

	return ErrRetryAttempts
}
