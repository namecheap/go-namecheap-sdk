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

// attemptFunc runs one HTTP attempt of an API call. It reports the HTTP status
// code observed (0 when the request never produced a response) and the error, so
// the retry driver can surface both to the observability layer.
type attemptFunc func(ctx context.Context) (status int, err error)

// do runs attempt through the resilience pipeline: rate limiter -> concurrency
// gate -> attempt -> retry policy. It retries an attempt only while its error is
// retryable for the given idempotency class (see shouldRetry), attempts remain,
// and the elapsed budget is not exhausted, sleeping an exponential, jittered,
// ctx-aware backoff between tries.
//
// idempotent distinguishes a safely-repeatable call from a charge-bearing one
// (domain create/renew/reactivate): a non-idempotent call is retried only on the
// pre-execution HTTP 405 rate-limit signal, never on ambiguous transport/server
// failures that may already have executed server-side.
//
// command and observeParams drive the observability layer. observeParams, when
// non-nil (i.e. when an observer is configured), is the credential-applied
// parameter map that do redacts once and threads onto every per-attempt
// RequestInfo. Observability ordering: the rate-limiter wait happens first, then
// OnRequest / slog request-start fires immediately before the HTTP send, then
// OnResponse / slog request-end fires after the attempt completes.
//
// A cancelled context aborts a limiter wait, a concurrency wait, or a backoff
// sleep promptly and returns the context error unwrapped. A terminal retry
// failure wraps the last real error as "after N attempts: <err>" so errors.Is
// and errors.As still reach the underlying *APIError.
func (c *Client) do(ctx context.Context, command string, observeParams map[string]string, idempotent bool, attempt attemptFunc) error {
	c.recordRequest(command)

	observe := c.observed()
	var redacted map[string]string
	if observe {
		redacted = redactParams(observeParams)
	}

	start := time.Now()
	var lastErr error
	attempts := 0

	for attempts < c.retry.MaxAttempts && time.Since(start) < c.retry.MaxElapsed {
		attempts++

		if err := c.waitLimiter(ctx, command, attempts, observe); err != nil {
			return err
		}

		if observe {
			c.safeOnRequest(RequestInfo{Command: command, Params: redacted, Attempt: attempts})
			c.logRequestStart(ctx, command, attempts, redacted)
		}

		attemptStart := time.Now()
		status, err := c.runAttempt(ctx, attempt)
		duration := time.Since(attemptStart)

		if err == nil {
			if observe {
				c.fireResponse(ctx, ResponseInfo{
					Command: command, Attempt: attempts, StatusCode: status, Duration: duration,
				}, 0)
			}
			return nil
		}
		lastErr = err

		errCode := errorCodeOf(err)
		if errCode != 0 {
			c.recordError(errCode)
		}

		retryable := shouldRetry(err, idempotent)
		delay, willRetry := c.retryPlan(retryable, attempts, start)

		if observe {
			c.fireResponse(ctx, ResponseInfo{
				Command: command, Attempt: attempts, StatusCode: status, Duration: duration,
				Err: err, ErrorCode: errCode, WillRetry: willRetry,
			}, delay)
		}

		if !retryable {
			return err
		}
		if !willRetry {
			break
		}
		c.recordRetry()
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}

	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

// waitLimiter blocks on the rate limiter (when enabled), records the wait time
// in the stats, and emits the limiter-wait slog event. It returns the context
// error if the wait is cancelled.
func (c *Client) waitLimiter(ctx context.Context, command string, attempt int, observe bool) error {
	if c.limiter == nil {
		return nil
	}
	waitStart := time.Now()
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	waited := time.Since(waitStart)
	c.recordLimiterWait(waited)
	if observe {
		c.logLimiterWait(ctx, command, attempt, waited)
	}
	return nil
}

// retryPlan reports the backoff delay before the next attempt and whether a
// retry will actually follow: only when the error is retryable, attempts remain,
// and the elapsed-time budget is not exhausted.
func (c *Client) retryPlan(retryable bool, attempts int, start time.Time) (time.Duration, bool) {
	if !retryable || attempts >= c.retry.MaxAttempts {
		return 0, false
	}
	delay, ok := c.nextDelay(attempts, time.Since(start))
	if !ok {
		return 0, false
	}
	return delay, true
}

// fireResponse fires the OnResponse hook and the request-end slog event for one
// attempt. Callers invoke it only when an observer is configured.
func (c *Client) fireResponse(ctx context.Context, info ResponseInfo, delay time.Duration) {
	c.safeOnResponse(info)
	c.logResponse(ctx, info, delay)
}

// runAttempt acquires the concurrency slot (if any), runs attempt, and releases
// the slot. It returns the context error promptly if the slot cannot be
// acquired before ctx is done.
func (c *Client) runAttempt(ctx context.Context, attempt attemptFunc) (int, error) {
	if c.sem == nil {
		return attempt(ctx)
	}
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return 0, ctx.Err()
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

// shouldRetry reports whether err is worth another attempt, given whether the
// call is idempotent.
//
// The internal HTTP 405 rate-limit sentinel (errRetryStatus) is always
// retryable: Namecheap returns 405 before it processes the request, so the call
// provably did not execute and resending it cannot double-apply an effect.
//
// For an idempotent call, any typed-retryable error (IsRetryable — transient
// server-side codes and transport timeouts) is also retried. For a
// non-idempotent, charge-bearing call it is NOT: a timeout or a server-side
// exception is ambiguous (the request may already have executed), so a blind
// resend could double-charge the account. Such calls fail fast on the first
// real error and let the caller reconcile.
func shouldRetry(err error, idempotent bool) bool {
	if errors.Is(err, errRetryStatus) {
		return true
	}
	if !idempotent {
		return false
	}
	return IsRetryable(err)
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
