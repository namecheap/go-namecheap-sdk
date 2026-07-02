package namecheap

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/time/rate"
)

// Resilience defaults applied when the corresponding option is nil or zero.
// They intentionally mirror Namecheap's published primary quota (20 requests
// per minute) and a polite, bounded retry schedule.
const (
	// defaultPerMinute is the token-bucket refill rate, matching Namecheap's
	// documented primary quota of 20 requests/minute.
	defaultPerMinute = 20
	// defaultMaxAttempts is the total number of attempts, including the first.
	defaultMaxAttempts = 4
	// defaultMaxElapsed caps the total wall-clock time spent across all retries.
	defaultMaxElapsed = 2 * time.Minute
	// defaultBaseDelay is the first backoff delay.
	defaultBaseDelay = 500 * time.Millisecond
	// defaultMaxDelay caps a single backoff delay.
	defaultMaxDelay = 30 * time.Second
)

// defaultUserAgent is sent on every request. When ClientOptions.UserAgent is
// set, it is appended after a space so support can identify the caller.
const defaultUserAgent = "go-namecheap-sdk/2 (+https://github.com/namecheap/go-namecheap-sdk)"

// errRetryStatus is an internal sentinel returned by an attempt when the server
// responds with HTTP 405, which Namecheap uses to signal rate limiting. The
// retry driver treats it as retryable; it never escapes to callers (a terminal
// failure wraps the last real error instead).
var errRetryStatus = errors.New("namecheap: retryable HTTP status")

// RateLimitOptions configures the client-side token-bucket limiter and the
// optional in-flight concurrency bound. The zero value selects the documented
// defaults (see PerMinute).
type RateLimitOptions struct {
	// PerMinute is the token-bucket rate in requests per minute. Zero selects
	// the default of 20 (Namecheap's primary quota). The bucket burst equals
	// PerMinute, so short bursts and genuine concurrency under the limit are not
	// throttled.
	PerMinute int
	// Disabled, when true, removes client-side rate limiting entirely.
	Disabled bool
	// MaxConcurrent bounds the number of in-flight requests. A value greater
	// than zero caps concurrency; zero leaves concurrency unbounded (limited
	// only by the token bucket). MaxConcurrent: 1 restores the fully serial
	// behavior of the pre-resilience SDK.
	MaxConcurrent int
}

// RetryOptions configures the exponential-backoff retry policy. Any zero field
// falls back to its documented default.
type RetryOptions struct {
	// MaxAttempts is the total number of attempts including the first. Zero
	// selects the default of 4.
	MaxAttempts int
	// MaxElapsed caps the total wall-clock time spent across all attempts and
	// their inter-attempt sleeps. Zero selects the default of 2 minutes.
	MaxElapsed time.Duration
	// BaseDelay is the first backoff delay; subsequent delays grow
	// exponentially. Zero selects the default of 500ms.
	BaseDelay time.Duration
	// MaxDelay caps any single backoff delay. Zero selects the default of 30s.
	MaxDelay time.Duration
}

// newLimiter builds the token-bucket limiter from opts, or returns nil when
// rate limiting is disabled. The burst equals the per-minute rate so a caller
// making genuinely concurrent requests under the quota is not throttled.
func newLimiter(opts *RateLimitOptions) *rate.Limiter {
	if opts != nil && opts.Disabled {
		return nil
	}
	perMinute := defaultPerMinute
	if opts != nil && opts.PerMinute > 0 {
		perMinute = opts.PerMinute
	}
	return rate.NewLimiter(rate.Every(time.Minute/time.Duration(perMinute)), perMinute)
}

// newSemaphore builds the concurrency-bounding semaphore from opts, or returns
// nil when concurrency is unbounded.
func newSemaphore(opts *RateLimitOptions) chan struct{} {
	if opts == nil || opts.MaxConcurrent <= 0 {
		return nil
	}
	return make(chan struct{}, opts.MaxConcurrent)
}

// resolveRetry fills any zero field of opts with its documented default.
func resolveRetry(opts *RetryOptions) RetryOptions {
	r := RetryOptions{
		MaxAttempts: defaultMaxAttempts,
		MaxElapsed:  defaultMaxElapsed,
		BaseDelay:   defaultBaseDelay,
		MaxDelay:    defaultMaxDelay,
	}
	if opts == nil {
		return r
	}
	if opts.MaxAttempts > 0 {
		r.MaxAttempts = opts.MaxAttempts
	}
	if opts.MaxElapsed > 0 {
		r.MaxElapsed = opts.MaxElapsed
	}
	if opts.BaseDelay > 0 {
		r.BaseDelay = opts.BaseDelay
	}
	if opts.MaxDelay > 0 {
		r.MaxDelay = opts.MaxDelay
	}
	return r
}

// resolveUserAgent appends a caller-supplied UA to the default one.
func resolveUserAgent(ua string) string {
	if ua == "" {
		return defaultUserAgent
	}
	return defaultUserAgent + " " + ua
}

// do runs attempt through the resilience pipeline: rate limiter -> concurrency
// gate -> attempt -> retry policy. It retries an attempt only while its error
// is retryable (IsRetryable or the internal 405 sentinel), attempts remain, and
// the elapsed budget is not exhausted, sleeping an exponential, jittered,
// ctx-aware backoff between tries.
//
// A cancelled context aborts a limiter wait, a concurrency wait, or a backoff
// sleep promptly and returns the context error unwrapped. A terminal retry
// failure wraps the last real error as "after N attempts: <err>" so errors.Is
// and errors.As still reach the underlying *APIError.
func (c *Client) do(ctx context.Context, attempt func(context.Context) error) error {
	start := time.Now()
	var lastErr error
	attempts := 0

	for attempts < c.retry.MaxAttempts && time.Since(start) < c.retry.MaxElapsed {
		attempts++

		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return err
			}
		}

		err := c.runAttempt(ctx, attempt)
		if err == nil {
			return nil
		}
		lastErr = err

		if !shouldRetry(err) {
			return err
		}
		if attempts >= c.retry.MaxAttempts {
			break
		}

		delay, ok := c.nextDelay(attempts, time.Since(start))
		if !ok {
			break
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}

	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

// runAttempt acquires the concurrency slot (if any), runs attempt, and releases
// the slot. It returns the context error promptly if the slot cannot be
// acquired before ctx is done.
func (c *Client) runAttempt(ctx context.Context, attempt func(context.Context) error) error {
	if c.sem == nil {
		return attempt(ctx)
	}
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.sem }()
	return attempt(ctx)
}

// nextDelay reports the ctx-agnostic backoff to wait before the next attempt,
// clamped so it never overshoots the remaining elapsed budget. ok is false when
// the budget is already exhausted.
func (c *Client) nextDelay(attempt int, elapsed time.Duration) (time.Duration, bool) {
	remaining := c.retry.MaxElapsed - elapsed
	if remaining <= 0 {
		return 0, false
	}
	d := backoff(c.retry.BaseDelay, c.retry.MaxDelay, attempt)
	if d > remaining {
		d = remaining
	}
	return d, true
}

// shouldRetry reports whether err is worth another attempt: either a typed
// retryable error (IsRetryable) or the internal HTTP 405 rate-limit sentinel.
func shouldRetry(err error) bool {
	return IsRetryable(err) || errors.Is(err, errRetryStatus)
}

// backoff computes the delay before the given attempt (1-based): an exponential
// base*2^(attempt-1) capped at maxDelay, then equal jitter — half the capped
// delay plus a uniform random point in the other half. Equal jitter keeps the
// per-attempt lower bound monotonically non-decreasing (so growth is
// observable) while still de-correlating concurrent clients. It uses the global
// math/rand source, which is safe for concurrent use.
func backoff(base, maxDelay time.Duration, attempt int) time.Duration {
	d := base
	for i := 1; i < attempt && d < maxDelay; i++ {
		d *= 2
	}
	if d <= 0 || d > maxDelay {
		d = maxDelay
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

// sleep waits for d or until ctx is done, returning ctx.Err() if the context is
// cancelled first.
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
