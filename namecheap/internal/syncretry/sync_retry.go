// Package syncretry provides a mutex-guarded retry mechanism with configurable
// delays between attempts.
package syncretry

import (
	"context"
	"errors"
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
	// sem is a capacity-1 semaphore used instead of a sync.Mutex so that a
	// goroutine waiting to enter the retry section can be released by a
	// cancelled context (a sync.Mutex.Lock cannot be interrupted).
	sem     chan struct{}
	options *Options
}

// NewSyncRetry returns a new SyncRetry configured with the given options.
func NewSyncRetry(options *Options) *SyncRetry {
	return &SyncRetry{
		sem:     make(chan struct{}, 1),
		options: options,
	}
}

// Do calls f and retries it after each configured delay when f returns
// ErrRetry.
//
// Deprecated: Do drives the retry loop without a context, so neither the
// inter-retry sleeps nor waiting to enter the retry section can be cancelled.
// Use DoContext, which threads a context.Context through both. Do is retained
// for backward compatibility and will be removed in v3.
func (sq *SyncRetry) Do(f func() error) error {
	return sq.DoContext(context.Background(), func(context.Context) error { return f() })
}

// DoContext calls f and, if f returns ErrRetry, acquires the internal
// semaphore and retries f after each delay configured in Options.Delays. It
// returns nil on the first successful call, any non-retriable error
// immediately, or ErrRetryAttempts when all retries are exhausted.
//
// DoContext is cancellable: if ctx is done it returns ctx.Err() instead of
// (a) starting a new attempt, (b) completing a pending inter-retry sleep, or
// (c) continuing to wait for the semaphore. The context is also passed to f so
// the caller can bind it to the in-flight work (e.g. an HTTP request).
func (sq *SyncRetry) DoContext(ctx context.Context, f func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := f(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrRetry) {
		return err
	}

	// Acquire the retry section, but abort if the context is cancelled while
	// we are queued behind another in-flight retry loop.
	select {
	case sq.sem <- struct{}{}:
		defer func() { <-sq.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	for _, delay := range sq.options.Delays {
		if err := sleep(ctx, time.Duration(delay)*time.Second); err != nil {
			return err
		}

		err = f(ctx)
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

// sleep waits for d or until ctx is done, returning ctx.Err() if the context
// is cancelled first.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
