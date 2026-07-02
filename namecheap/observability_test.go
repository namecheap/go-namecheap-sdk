package namecheap

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- test helpers -----------------------------------------------------------

// capturedRecord is a flattened slog.Record kept by captureHandler for attr
// assertions.
type capturedRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]slog.Value
}

// captureHandler is a slog.Handler that records every emitted record (with its
// attrs flattened) for assertions. It is safe for concurrent use.
type captureHandler struct {
	mu      *sync.Mutex
	records *[]capturedRecord
	attrs   []slog.Attr
}

func newCaptureHandler() (*captureHandler, *[]capturedRecord) {
	recs := &[]capturedRecord{}
	return &captureHandler{mu: &sync.Mutex{}, records: recs}, recs
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := capturedRecord{Level: r.Level, Message: r.Message, Attrs: map[string]slog.Value{}}
	for _, a := range h.attrs {
		rec.Attrs[a.Key] = a.Value
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value
		return true
	})
	h.mu.Lock()
	*h.records = append(*h.records, rec)
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(as []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(as))
	merged = append(merged, h.attrs...)
	merged = append(merged, as...)
	return &captureHandler{mu: h.mu, records: h.records, attrs: merged}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// --- hooks fire per attempt with correct numbering --------------------------

// TestHooksFirePerAttempt asserts OnRequest/OnResponse fire once per attempt
// with 1-based Attempt numbering across a 405-retry sequence, and that the final
// success carries WillRetry=false while the retried failures carry WillRetry=true.
func TestHooksFirePerAttempt(t *testing.T) {
	t.Parallel()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusMethodNotAllowed) // 405 twice
			return
		}
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	var mu sync.Mutex
	var reqAttempts, respAttempts []int
	var respStatuses []int
	var respWillRetry []bool

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		o.OnRequest = func(info RequestInfo) {
			mu.Lock()
			reqAttempts = append(reqAttempts, info.Attempt)
			mu.Unlock()
		}
		o.OnResponse = func(info ResponseInfo) {
			mu.Lock()
			respAttempts = append(respAttempts, info.Attempt)
			respStatuses = append(respStatuses, info.StatusCode)
			respWillRetry = append(respWillRetry, info.WillRetry)
			mu.Unlock()
		}
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "test.retry"}, &obj)
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, reqAttempts, "OnRequest must fire once per attempt, 1-based")
	assert.Equal(t, []int{1, 2, 3}, respAttempts, "OnResponse must fire once per attempt, 1-based")
	assert.Equal(t, []int{405, 405, 200}, respStatuses, "status per attempt")
	assert.Equal(t, []bool{true, true, false}, respWillRetry, "WillRetry per attempt")
}

// TestOnResponseErrorCode asserts OnResponse carries the numeric error code for
// an API-error response.
func TestOnResponseErrorCode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(apiErrorXML(2019166, "Domain not found")))
	}))
	defer server.Close()

	var got ResponseInfo
	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: 3, BaseDelay: time.Millisecond}
		o.OnResponse = func(info ResponseInfo) { got = info }
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "namecheap.domains.getInfo"}, &obj)
	assert.Error(t, err)
	assert.Equal(t, 2019166, got.ErrorCode)
	assert.False(t, got.WillRetry, "a permanent error must not schedule a retry")
}

// --- REDACTION (must-pass AC) -----------------------------------------------

// TestRedactionNeverLeaksSecrets configures a client with a distinctive ApiKey
// and drives success, API-error, retry and change-password paths while
// collecting every string the hooks receive and every byte the slog handler
// emits. It asserts the secret values appear ZERO times and "***" appears.
func TestRedactionNeverLeaksSecrets(t *testing.T) {
	const (
		secretAPIKey = "SUPERSECRETKEY-abc123XYZ"
		oldPassword  = "OLDPASS-zzz789QQQ"
		newPassword  = "NEWPASS-www456RRR"
	)

	var retryHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form := string(body)
		switch {
		case strings.Contains(form, "test.error"):
			_, _ = w.Write([]byte(apiErrorXML(2019166, "Domain not found")))
		case strings.Contains(form, "test.retry"):
			// Rate-limit signal on the first attempt, then succeed.
			if atomic.AddInt32(&retryHits, 1) == 1 {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(okXML))
		case strings.Contains(form, "changePassword"):
			_, _ = w.Write([]byte(`<?xml version="1.0"?><ApiResponse Status="OK"><CommandResponse><UserChangePasswordResult Success="true" UserID="42"/></CommandResponse></ApiResponse>`))
		default:
			_, _ = w.Write([]byte(okXML))
		}
	}))
	defer server.Close()

	// Collect every string the hooks receive.
	var mu sync.Mutex
	var seen []string
	collect := func(ss ...string) {
		mu.Lock()
		seen = append(seen, ss...)
		mu.Unlock()
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := NewClient(&ClientOptions{
		UserName: "acct-user", ApiUser: "acct-user", ApiKey: secretAPIKey, ClientIp: "10.0.0.1",
		RateLimit: &RateLimitOptions{Disabled: true},
		Retry:     &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond},
		Logger:    logger,
		OnRequest: func(info RequestInfo) {
			collect(info.Command)
			for k, v := range info.Params {
				collect(k, v)
			}
		},
		OnResponse: func(info ResponseInfo) {
			collect(info.Command)
			if info.Err != nil {
				collect(info.Err.Error())
			}
		},
	})
	client.BaseURL = server.URL

	// Success path.
	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "test.ok"}, &obj)
	assert.NoError(t, err)

	// API-error path.
	_, _ = client.DoXMLWithContext(context.Background(), map[string]string{"Command": "test.error"}, &obj)

	// Retry path (405 once, then success): redaction must hold across attempts.
	_, err = client.DoXMLWithContext(context.Background(), map[string]string{"Command": "test.retry"}, &obj)
	assert.NoError(t, err)

	// Change-password path (exercises OldPassword/NewPassword redaction).
	_, err = client.Users.ChangePasswordWithContext(context.Background(), &UsersChangePasswordArgs{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	})
	assert.NoError(t, err)

	mu.Lock()
	hookBlob := strings.Join(seen, "\x00")
	mu.Unlock()
	logBlob := logBuf.String()
	all := hookBlob + "\x00" + logBlob

	for _, secret := range []string{secretAPIKey, oldPassword, newPassword} {
		assert.NotContains(t, all, secret, "secret value must never reach an observability surface")
	}
	assert.Contains(t, hookBlob, redactedValue, "hooks must show the redaction marker")
	assert.Contains(t, logBlob, redactedValue, "slog output must show the redaction marker")
}

// TestRedactParamsCopyAndReplacement unit-tests redactParams directly: it
// replaces every secret key, leaves the rest, and never mutates the caller's map.
func TestRedactParamsCopyAndReplacement(t *testing.T) {
	t.Parallel()

	in := map[string]string{
		"Command":     "namecheap.users.changePassword",
		"ApiKey":      "secret-key",
		"OldPassword": "old-secret",
		"NewPassword": "new-secret",
		"ResetCode":   "reset-secret",
		"ClientIp":    "10.0.0.1",
	}
	out := redactParams(in)

	assert.Equal(t, redactedValue, out["ApiKey"])
	assert.Equal(t, redactedValue, out["OldPassword"])
	assert.Equal(t, redactedValue, out["NewPassword"])
	assert.Equal(t, redactedValue, out["ResetCode"])
	assert.Equal(t, "namecheap.users.changePassword", out["Command"])
	assert.Equal(t, "10.0.0.1", out["ClientIp"])

	// The caller's map is untouched.
	assert.Equal(t, "secret-key", in["ApiKey"])
	assert.Equal(t, "old-secret", in["OldPassword"])
	// Mutating the copy does not affect the input.
	out["ClientIp"] = "mutated"
	assert.Equal(t, "10.0.0.1", in["ClientIp"])
}

// --- panic safety -----------------------------------------------------------

// TestHookPanicRecovered asserts a panicking hook does not crash the caller or
// abort the request, and is logged when a Logger is set.
func TestHookPanicRecovered(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Logger = logger
		o.OnRequest = func(RequestInfo) { panic("boom-request") }
		o.OnResponse = func(ResponseInfo) { panic("boom-response") }
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err, "a panicking hook must not fail the request")

	logged := logBuf.String()
	assert.Contains(t, logged, "hook panicked")
	assert.Contains(t, logged, "OnRequest")
	assert.Contains(t, logged, "OnResponse")
}

// TestHookPanicWithoutLogger asserts a panicking hook is still recovered when no
// Logger is configured (the request succeeds).
func TestHookPanicWithoutLogger(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.OnResponse = func(ResponseInfo) { panic("boom") }
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err)
}

// --- slog attrs -------------------------------------------------------------

// TestSlogAttrsPresent asserts the request-end slog records carry the documented
// attributes: command, duration, attempt, status (and error_code on failure).
func TestSlogAttrsPresent(t *testing.T) {
	t.Parallel()

	handler, recs := newCaptureHandler()
	logger := slog.New(handler)

	// Success path.
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer okServer.Close()

	client := newResilienceClient(okServer.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Logger = logger
	})
	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "namecheap.domains.getList"}, &obj)
	assert.NoError(t, err)

	completed := findRecord(*recs, "namecheap request completed")
	if !assert.NotNil(t, completed, "a request-completed record must be emitted") {
		t.FailNow()
	}
	assert.Equal(t, "namecheap.domains.getList", completed.Attrs["command"].String())
	assert.Equal(t, int64(1), completed.Attrs["attempt"].Int64())
	assert.Equal(t, int64(200), completed.Attrs["status"].Int64())
	assert.Equal(t, slog.KindDuration, completed.Attrs["duration"].Kind())

	// Failure path (error_code present).
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(apiErrorXML(2019166, "Domain not found")))
	}))
	defer errServer.Close()

	errClient := newResilienceClient(errServer.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: 1}
		o.Logger = logger
	})
	_, _ = errClient.DoXMLWithContext(context.Background(), map[string]string{"Command": "namecheap.domains.getInfo"}, &obj)

	failed := findRecord(*recs, "namecheap request failed")
	if !assert.NotNil(t, failed, "a request-failed record must be emitted") {
		t.FailNow()
	}
	assert.Equal(t, int64(2019166), failed.Attrs["error_code"].Int64())
	assert.Equal(t, slog.LevelWarn, failed.Level)
}

func findRecord(recs []capturedRecord, msg string) *capturedRecord {
	for i := range recs {
		if recs[i].Message == msg {
			return &recs[i]
		}
	}
	return nil
}

// --- zero-alloc guard -------------------------------------------------------

// noopAttempt is a pre-built successful attempt used by the zero-alloc guards so
// building the closure is not measured.
func noopAttempt(context.Context) (int, error) { return http.StatusOK, nil }

// TestZeroAllocWhenNoObserver asserts the observability path adds no allocations
// when neither a Logger nor a hook is configured.
func TestZeroAllocWhenNoObserver(t *testing.T) {
	client := NewClient(&ClientOptions{
		UserName:  ncUserName,
		ApiKey:    ncAPIKey,
		RateLimit: &RateLimitOptions{Disabled: true},
		Retry:     &RetryOptions{MaxAttempts: 1},
	})
	ctx := context.Background()
	// Warm up the stats map key so a steady-state call inserts nothing.
	_ = client.do(ctx, "test.command", nil, true, noopAttempt)

	allocs := testing.AllocsPerRun(200, func() {
		_ = client.do(ctx, "test.command", nil, true, noopAttempt)
	})
	assert.Zero(t, allocs, "no allocations expected on the observability path when unobserved")
}

// BenchmarkDoNoObserver guards the zero-overhead-when-unobserved property.
func BenchmarkDoNoObserver(b *testing.B) {
	client := NewClient(&ClientOptions{
		UserName:  ncUserName,
		ApiKey:    ncAPIKey,
		RateLimit: &RateLimitOptions{Disabled: true},
		Retry:     &RetryOptions{MaxAttempts: 1},
	})
	ctx := context.Background()
	_ = client.do(ctx, "test.command", nil, true, noopAttempt)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.do(ctx, "test.command", nil, true, noopAttempt)
	}
}

// --- Stats ------------------------------------------------------------------

// TestStatsAccurateConcurrent hammers the client from 100 goroutines and asserts
// the request counter is exact. Run under -race for the concurrency guarantee.
func TestStatsAccurateConcurrent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
	})

	const goroutines = 100
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var obj testResponse
			_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	stats := client.Stats()
	assert.Equal(t, int64(goroutines), stats.RequestsByCommand["x"])
	assert.Equal(t, int64(0), stats.Retries)
	assert.Empty(t, stats.ErrorsByCode)
}

// TestStatsCountsRetriesAndErrors asserts retries and per-code errors are
// counted across a persistent retryable failure.
func TestStatsCountsRetriesAndErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(apiErrorXML(5050900, "Server exception")))
	}))
	defer server.Close()

	const attempts = 3
	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: attempts, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.Error(t, err)

	stats := client.Stats()
	assert.Equal(t, int64(1), stats.RequestsByCommand["x"])
	assert.Equal(t, int64(attempts-1), stats.Retries, "attempts beyond the first are retries")
	assert.Equal(t, int64(attempts), stats.ErrorsByCode[5050900], "each failed attempt is counted by code")
}

// TestStatsSnapshotIsCopy asserts the returned Stats is a deep copy: mutating it
// does not affect the client, and later calls reflect subsequent activity.
func TestStatsSnapshotIsCopy(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err)

	snap := client.Stats()
	assert.Equal(t, int64(1), snap.RequestsByCommand["x"])

	// Mutating the snapshot must not affect the client's live counters.
	snap.RequestsByCommand["x"] = 999
	snap.ErrorsByCode[42] = 7
	fresh := client.Stats()
	assert.Equal(t, int64(1), fresh.RequestsByCommand["x"], "snapshot mutation must not leak into the client")
	assert.Empty(t, fresh.ErrorsByCode)
}

// TestStatsQuotaRemaining asserts QuotaRemaining reflects the limiter tokens and
// is 0 when rate limiting is disabled.
func TestStatsQuotaRemaining(t *testing.T) {
	t.Parallel()

	withLimiter := NewClient(&ClientOptions{UserName: ncUserName, RateLimit: &RateLimitOptions{PerMinute: 20}})
	assert.Positive(t, withLimiter.Stats().QuotaRemaining, "a fresh limiter bucket has tokens")

	disabled := NewClient(&ClientOptions{UserName: ncUserName, RateLimit: &RateLimitOptions{Disabled: true}})
	assert.Equal(t, 0, disabled.Stats().QuotaRemaining, "disabled limiter reports 0 quota")
}
