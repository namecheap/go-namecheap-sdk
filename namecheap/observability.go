package namecheap

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// redactedValue replaces every secret parameter value in the redacted copy of
// the request parameters exposed to observers (hooks and slog).
const redactedValue = "***"

// secretParamKeys is the set of request-parameter keys whose values are secrets
// and MUST never be exposed to an observability hook, a slog record, or any
// other diagnostic surface. Their values are replaced with redactedValue before
// a RequestInfo is built or a slog record is emitted.
//
// The set is deliberately a map so it is cheap to extend: add the new key here
// and every observability surface redacts it automatically. It currently covers
// the credential the SDK injects on every call (ApiKey) and the credential-like
// fields the users.changePassword call sends (NewPassword, OldPassword,
// ResetCode).
var secretParamKeys = map[string]struct{}{
	"ApiKey":      {},
	"NewPassword": {},
	"OldPassword": {},
	"ResetCode":   {},
}

// RequestInfo describes a single outgoing HTTP attempt for an API call. It is
// passed to ClientOptions.OnRequest immediately before the request is sent (and
// after any rate-limiter wait). One RequestInfo is delivered per attempt, so a
// retried call yields several with an increasing Attempt.
type RequestInfo struct {
	// Command is the Namecheap command being invoked, e.g.
	// "namecheap.domains.getInfo".
	Command string
	// Params is a REDACTED copy of the request parameters: every secret key
	// (see the package's secret-key set: ApiKey, NewPassword, OldPassword,
	// ResetCode) has its value replaced with "***". It is never the live
	// parameter map and never carries a credential. Treat it as read-only.
	Params map[string]string
	// Attempt is the 1-based attempt number: 1 on the first try, 2 on the first
	// retry, and so on.
	Attempt int
}

// ResponseInfo describes the outcome of a single HTTP attempt for an API call.
// It is passed to ClientOptions.OnResponse after the attempt completes, carrying
// its duration, outcome and whether a retry will follow. One ResponseInfo is
// delivered per attempt.
type ResponseInfo struct {
	// Command is the Namecheap command that was invoked.
	Command string
	// Attempt is the 1-based attempt number this outcome belongs to.
	Attempt int
	// StatusCode is the HTTP status code observed on this attempt, or 0 when the
	// request never produced a response (a transport error).
	StatusCode int
	// Duration is the wall-clock time this attempt took, measured around the
	// HTTP round trip (it excludes the preceding rate-limiter wait).
	Duration time.Duration
	// Err is the error this attempt produced, or nil on success. For a terminal
	// API rejection it is an *APIError; for the pre-execution HTTP 405 rate-limit
	// signal it is an internal retry sentinel.
	Err error
	// ErrorCode is the numeric Namecheap error code from Err when Err is an
	// *APIError, and 0 otherwise (success, transport error, or the HTTP 405
	// rate-limit signal).
	ErrorCode int
	// WillRetry reports whether the client will make another attempt after this
	// one: true only when the error is retryable, attempts remain, and the
	// elapsed-time budget is not exhausted.
	WillRetry bool
}

// Stats is a point-in-time snapshot of a client's cumulative observability
// counters, returned by Client.Stats. Its maps are copies: mutating them (or the
// snapshot) never affects the client. Build a Prometheus/OTel exporter on top of
// it without the SDK depending on either.
type Stats struct {
	// RequestsByCommand counts the API calls made per command (one per logical
	// call, not per attempt; retries are counted separately in Retries).
	RequestsByCommand map[string]int64
	// ErrorsByCode counts failed attempts keyed by Namecheap numeric error code.
	// Transport-level and HTTP 405 failures have no numeric code and are not
	// counted here.
	ErrorsByCode map[int]int64
	// Retries is the total number of retry attempts made across all calls (i.e.
	// attempts beyond the first).
	Retries int64
	// TotalLimiterWait is the cumulative time spent blocked in the client-side
	// rate limiter across all attempts.
	TotalLimiterWait time.Duration
	// QuotaRemaining is a best-effort estimate of the tokens currently available
	// in the rate limiter's minute bucket. It is a snapshot of a live,
	// continuously refilling value and is 0 when rate limiting is disabled; treat
	// it as an estimate, not an exact remaining-quota guarantee.
	QuotaRemaining int
}

// redactParams returns a new map with every secret key's value replaced by
// "***" (see secretParamKeys). It never mutates the caller's map and always
// returns a fresh copy, so the result is safe to hand to an untrusted hook.
func redactParams(params map[string]string) map[string]string {
	out := make(map[string]string, len(params))
	for k, v := range params {
		if _, secret := secretParamKeys[k]; secret {
			out[k] = redactedValue
		} else {
			out[k] = v
		}
	}
	return out
}

// observed reports whether any observability surface is configured. When it is
// false the request path skips all redaction and RequestInfo/ResponseInfo
// construction entirely, so an unobserved client pays no observability overhead.
func (c *Client) observed() bool {
	return c.onRequest != nil || c.onResponse != nil || c.logger != nil
}

// withCredentials returns a copy of body with the client's credentials applied,
// matching exactly what NewRequestWithContext puts on the wire. It is used only
// to build the redacted view handed to observers, so the redacted ApiKey is
// visible (as "***") to hooks and slog. It never mutates body.
func (c *Client) withCredentials(body map[string]string) map[string]string {
	out := make(map[string]string, len(body)+4)
	for k, v := range body {
		out[k] = v
	}
	out["Username"] = c.ClientOptions.UserName
	out["ApiKey"] = c.ClientOptions.ApiKey
	out["ApiUser"] = c.ClientOptions.ApiUser
	out["ClientIp"] = c.ClientOptions.ClientIp
	return out
}

// safeOnRequest invokes the OnRequest hook (if any) with panic protection: a
// panicking hook is recovered and, when a Logger is configured, logged; it never
// crashes the caller or aborts the request.
func (c *Client) safeOnRequest(info RequestInfo) {
	if c.onRequest == nil {
		return
	}
	defer c.recoverHook("OnRequest")
	c.onRequest(info)
}

// safeOnResponse invokes the OnResponse hook (if any) with the same panic
// protection as safeOnRequest.
func (c *Client) safeOnResponse(info ResponseInfo) {
	if c.onResponse == nil {
		return
	}
	defer c.recoverHook("OnResponse")
	c.onResponse(info)
}

// recoverHook recovers a panicking observability hook and logs it when a Logger
// is set, so an ill-behaved hook can never crash the caller or abort a request.
func (c *Client) recoverHook(name string) {
	if r := recover(); r != nil && c.logger != nil {
		c.logger.Error("namecheap: observability hook panicked",
			slog.String("hook", name), slog.Any("panic", r))
	}
}

// errorCodeOf returns the numeric Namecheap error code carried by err when it is
// (or wraps) an *APIError, and 0 otherwise.
func errorCodeOf(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Number
	}
	return 0
}

// retryReason returns a short, human-readable reason an attempt is being
// retried, for slog diagnostics.
func retryReason(err error) string {
	switch {
	case errors.Is(err, errRetryStatus):
		return "http 405 rate limit"
	case errorCodeOf(err) != 0:
		return "transient server error"
	default:
		return "transient transport error"
	}
}

// recordRequest counts one logical API call for command.
func (c *Client) recordRequest(command string) {
	c.statsMu.Lock()
	c.statRequests[command]++
	c.statsMu.Unlock()
}

// recordError counts one failed attempt carrying the given numeric error code.
func (c *Client) recordError(code int) {
	c.statsMu.Lock()
	c.statErrors[code]++
	c.statsMu.Unlock()
}

// recordRetry counts one retry (an attempt beyond the first).
func (c *Client) recordRetry() {
	c.statsMu.Lock()
	c.statRetries++
	c.statsMu.Unlock()
}

// recordLimiterWait adds d to the cumulative rate-limiter wait time.
func (c *Client) recordLimiterWait(d time.Duration) {
	c.statsMu.Lock()
	c.statLimiterWait += d
	c.statsMu.Unlock()
}

// Stats returns a point-in-time snapshot of the client's observability
// counters. The returned maps are deep copies, so mutating the snapshot never
// affects the client, and the call is safe for concurrent use.
func (c *Client) Stats() Stats {
	c.statsMu.Lock()
	requests := make(map[string]int64, len(c.statRequests))
	for k, v := range c.statRequests {
		requests[k] = v
	}
	errorsByCode := make(map[int]int64, len(c.statErrors))
	for k, v := range c.statErrors {
		errorsByCode[k] = v
	}
	snapshot := Stats{
		RequestsByCommand: requests,
		ErrorsByCode:      errorsByCode,
		Retries:           c.statRetries,
		TotalLimiterWait:  c.statLimiterWait,
	}
	c.statsMu.Unlock()

	if c.limiter != nil {
		snapshot.QuotaRemaining = int(c.limiter.Tokens())
	}
	return snapshot
}

// logRequestStart emits the request-start slog event (Debug) for one attempt.
// The params it logs are already redacted.
func (c *Client) logRequestStart(ctx context.Context, command string, attempt int, params map[string]string) {
	if c.logger == nil {
		return
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "namecheap request start",
		slog.String("command", command),
		slog.Int("attempt", attempt),
		slog.Any("params", params),
	)
}

// logLimiterWait emits the rate-limiter-wait slog event (Debug).
func (c *Client) logLimiterWait(ctx context.Context, command string, attempt int, waited time.Duration) {
	if c.logger == nil {
		return
	}
	c.logger.LogAttrs(ctx, slog.LevelDebug, "namecheap limiter wait",
		slog.String("command", command),
		slog.Int("attempt", attempt),
		slog.Duration("duration", waited),
	)
}

// logResponse emits the request-end slog event: Info on success, Warn on
// failure. When the failure will be retried the record also carries the retry
// delay and reason (folding the "retry" event into the same record). It never
// logs an unredacted parameter.
func (c *Client) logResponse(ctx context.Context, info ResponseInfo, delay time.Duration) {
	if c.logger == nil {
		return
	}
	if info.Err == nil {
		c.logger.LogAttrs(ctx, slog.LevelInfo, "namecheap request completed",
			slog.String("command", info.Command),
			slog.Int("attempt", info.Attempt),
			slog.Duration("duration", info.Duration),
			slog.Int("status", info.StatusCode),
		)
		return
	}

	attrs := []slog.Attr{
		slog.String("command", info.Command),
		slog.Int("attempt", info.Attempt),
		slog.Duration("duration", info.Duration),
		slog.Int("status", info.StatusCode),
		slog.Int("error_code", info.ErrorCode),
		slog.String("error", info.Err.Error()),
	}
	msg := "namecheap request failed"
	if info.WillRetry {
		msg = "namecheap request failed, retrying"
		attrs = append(attrs,
			slog.Duration("retry_delay", delay),
			slog.String("retry_reason", retryReason(info.Err)),
		)
	}
	c.logger.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}
