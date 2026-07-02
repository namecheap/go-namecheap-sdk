package namecheap

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// testResponse is a minimal struct the resilience tests decode successful
// responses into (xml.Unmarshal cannot target a bare interface{}).
type testResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	Status  string   `xml:"Status,attr"`
}

const okXML = `<?xml version="1.0"?><ApiResponse Status="OK"></ApiResponse>`

// apiErrorXML renders an error-envelope response carrying a single numeric code.
func apiErrorXML(number int, message string) string {
	return fmt.Sprintf(
		`<?xml version="1.0"?><ApiResponse Status="ERROR"><Errors><Error Number="%d">%s</Error></Errors></ApiResponse>`,
		number, message,
	)
}

// newResilienceClient builds a client with the standard test credentials, an
// optional mutation of its options, and BaseURL pointed at baseURL.
func newResilienceClient(baseURL string, mutate func(*ClientOptions)) *Client {
	opts := &ClientOptions{
		UserName: ncUserName, ApiUser: ncAPIUser, ApiKey: ncAPIKey, ClientIp: ncClientIP,
	}
	if mutate != nil {
		mutate(opts)
	}
	c := NewClient(opts)
	if baseURL != "" {
		c.BaseURL = baseURL
	}
	return c
}

// recordingRT is an http.RoundTripper that records calls and the User-Agent,
// returning a canned response without touching the network.
type recordingRT struct {
	body   string
	status int

	mu    sync.Mutex
	calls int
	ua    string
}

func (rt *recordingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.calls++
	rt.ua = req.Header.Get("User-Agent")
	rt.mu.Unlock()

	status := rt.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Header:     make(http.Header),
	}, nil
}

func (rt *recordingRT) count() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.calls
}

func (rt *recordingRT) lastUserAgent() string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.ua
}

// TestConcurrentRequestsOverlap asserts that two requests are in flight
// simultaneously under the default limit — proving the global-mutex
// serialization is gone.
func TestConcurrentRequestsOverlap(t *testing.T) {
	t.Parallel()
	const n = 2
	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	allIn := make(chan struct{})
	var once sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		reached := inFlight == n
		mu.Unlock()
		if reached {
			once.Do(func() { close(allIn) })
		}
		// Block until every goroutine is simultaneously in flight (or a safety
		// timeout), which can only happen if the requests overlap.
		select {
		case <-allIn:
		case <-time.After(2 * time.Second):
		}
		_, _ = w.Write([]byte(okXML))
		mu.Lock()
		inFlight--
		mu.Unlock()
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, nil) // default limiter (burst 20), unbounded concurrency

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var obj testResponse
			_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "test"}, &obj)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, n, maxInFlight, "requests should have been in flight simultaneously")
}

// TestMaxConcurrentSerializes asserts MaxConcurrent:1 restores the old,
// strictly serial behavior.
func TestMaxConcurrentSerializes(t *testing.T) {
	t.Parallel()
	const n = 2
	var active, violations int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&active, 1) > 1 {
			atomic.AddInt32(&violations, 1)
		}
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{MaxConcurrent: 1, Disabled: true}
	})

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var obj testResponse
			_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	assert.Equal(t, int32(0), atomic.LoadInt32(&violations), "MaxConcurrent:1 must serialize handlers")
	assert.GreaterOrEqual(t, elapsed, 80*time.Millisecond, "serialized handlers should run back-to-back")
}

// TestTokenBucketRefillWaits asserts the (N+1)th request waits roughly one
// refill interval rather than a server round-trip. A burst-1 limiter is injected
// directly so the wait is observable in tens of milliseconds.
func TestTokenBucketRefillWaits(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	const refill = 50 * time.Millisecond
	client := newResilienceClient(server.URL, nil)
	client.limiter = rate.NewLimiter(rate.Every(refill), 1) // starts with one token

	ctx := context.Background()
	var obj testResponse

	start := time.Now()
	_, err := client.DoXMLWithContext(ctx, map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err)
	first := time.Since(start)

	start2 := time.Now()
	_, err = client.DoXMLWithContext(ctx, map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err)
	second := time.Since(start2)

	assert.Less(t, first, refill, "first request should not wait (bucket starts full)")
	assert.GreaterOrEqual(t, second, refill/2, "second request should wait ~one refill interval")
	assert.Less(t, second, 3*refill, "second request should not wait excessively")
}

// TestBackoffGrowsAndBounded asserts a persistently retryable response is tried
// exactly MaxAttempts times with growing, bounded inter-attempt gaps.
func TestBackoffGrowsAndBounded(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var times []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		times = append(times, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	const (
		base     = 20 * time.Millisecond
		capDelay = 1 * time.Second
		attempts = 4
	)
	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: attempts, BaseDelay: base, MaxDelay: capDelay}
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.Error(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, times, attempts, "should try exactly MaxAttempts times")

	gaps := make([]time.Duration, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		g := times[i].Sub(times[i-1])
		gaps = append(gaps, g)
		assert.Positive(t, g, "each gap must be a real backoff sleep")
		assert.LessOrEqual(t, g, capDelay+200*time.Millisecond, "gap must stay within MaxDelay (+slack)")
	}
	// Equal jitter keeps the per-attempt lower bound non-decreasing, so the last
	// gap must exceed the first (jitter-tolerant growth check).
	assert.Greater(t, gaps[len(gaps)-1], gaps[0], "backoff should grow across attempts")
}

// TestNonRetryableSingleAttempt asserts a permanent error (domain not found)
// is never retried.
func TestNonRetryableSingleAttempt(t *testing.T) {
	t.Parallel()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(apiErrorXML(2019166, "Domain not found")))
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: 5, BaseDelay: time.Millisecond}
	})

	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "namecheap.domains.getInfo"}, &obj)
	assert.Error(t, err)
	var apiErr *APIError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 2019166, apiErr.Number)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "non-retryable error must not be retried")
}

// TestContextCancelsLimiterWait asserts a cancelled context interrupts a
// blocked rate-limiter wait promptly.
func TestContextCancelsLimiterWait(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okXML))
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, nil)
	// A drained limiter that refills only hourly blocks the very first Wait.
	l := rate.NewLimiter(rate.Every(time.Hour), 1)
	l.Allow() // consume the only token
	client.limiter = l

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	var obj testResponse
	start := time.Now()
	_, err := client.DoXMLWithContext(ctx, map[string]string{"Command": "x"}, &obj)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, elapsed, 100*time.Millisecond, "limiter wait should abort promptly")
}

// TestContextCancelsBackoffSleep asserts a cancelled context interrupts a
// backoff sleep promptly.
func TestContextCancelsBackoffSleep(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := newResilienceClient(server.URL, func(o *ClientOptions) {
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: 10 * time.Second, MaxDelay: 10 * time.Second}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	var obj testResponse
	start := time.Now()
	_, err := client.DoXMLWithContext(ctx, map[string]string{"Command": "x"}, &obj)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, elapsed, 100*time.Millisecond, "backoff sleep should abort promptly")
}

// TestTerminalErrorUnwrapsAPIError asserts a persistent retryable *APIError is
// reachable via errors.As after the retries are exhausted.
func TestTerminalErrorUnwrapsAPIError(t *testing.T) {
	t.Parallel()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
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
	assert.Contains(t, err.Error(), fmt.Sprintf("after %d attempts:", attempts))
	assert.NotContains(t, err.Error(), "retry limit exceeded")

	var apiErr *APIError
	assert.True(t, errors.As(err, &apiErr), "terminal error should unwrap to *APIError")
	assert.Equal(t, 5050900, apiErr.Number)
	assert.Equal(t, int32(attempts), atomic.LoadInt32(&calls))
}

// TestOptionDefaults asserts nil/zero options resolve to the documented
// defaults and that explicit values are honored.
func TestOptionDefaults(t *testing.T) {
	t.Parallel()

	t.Run("nil_retry_and_ratelimit", func(t *testing.T) {
		t.Parallel()
		client := NewClient(&ClientOptions{UserName: ncUserName})
		assert.Equal(t, defaultMaxAttempts, client.retry.MaxAttempts)
		assert.Equal(t, defaultMaxElapsed, client.retry.MaxElapsed)
		assert.Equal(t, defaultBaseDelay, client.retry.BaseDelay)
		assert.Equal(t, defaultMaxDelay, client.retry.MaxDelay)
		if assert.NotNil(t, client.limiter) {
			assert.Equal(t, defaultPerMinute, client.limiter.Burst())
			assert.InDelta(t, float64(rate.Every(time.Minute/defaultPerMinute)), float64(client.limiter.Limit()), 1e-9)
		}
		assert.Nil(t, client.sem, "concurrency unbounded by default")
		assert.Equal(t, defaultUserAgent, client.userAgent)
	})

	t.Run("disabled_limiter", func(t *testing.T) {
		t.Parallel()
		client := NewClient(&ClientOptions{UserName: ncUserName, RateLimit: &RateLimitOptions{Disabled: true}})
		assert.Nil(t, client.limiter)
	})

	t.Run("custom_values_preserved", func(t *testing.T) {
		t.Parallel()
		client := NewClient(&ClientOptions{
			UserName:  ncUserName,
			RateLimit: &RateLimitOptions{PerMinute: 120, MaxConcurrent: 4},
			Retry:     &RetryOptions{MaxAttempts: 7},
		})
		assert.Equal(t, 120, client.limiter.Burst())
		if assert.NotNil(t, client.sem) {
			assert.Equal(t, 4, cap(client.sem))
		}
		assert.Equal(t, 7, client.retry.MaxAttempts)
		// Unset retry fields fall back to defaults.
		assert.Equal(t, defaultBaseDelay, client.retry.BaseDelay)
	})
}

// TestUserAgentHeader asserts the default UA and the appended-UA behavior.
func TestUserAgentHeader(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		rt := &recordingRT{body: okXML}
		client := NewClient(&ClientOptions{UserName: ncUserName, Transport: rt})
		var obj testResponse
		_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, defaultUserAgent, rt.lastUserAgent())
	})

	t.Run("appended", func(t *testing.T) {
		t.Parallel()
		rt := &recordingRT{body: okXML}
		client := NewClient(&ClientOptions{UserName: ncUserName, Transport: rt, UserAgent: "my-app/1.2"})
		var obj testResponse
		_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
		assert.NoError(t, err)
		assert.Equal(t, defaultUserAgent+" my-app/1.2", rt.lastUserAgent())
	})
}

// TestHTTPClientHonored asserts an injected *http.Client is used for requests.
func TestHTTPClientHonored(t *testing.T) {
	t.Parallel()
	rt := &recordingRT{body: okXML}
	client := NewClient(&ClientOptions{UserName: ncUserName, HTTPClient: &http.Client{Transport: rt}})
	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err)
	assert.Equal(t, 1, rt.count())
}

// TestTransportHonored asserts an injected RoundTripper is applied onto the
// effective HTTP client.
func TestTransportHonored(t *testing.T) {
	t.Parallel()
	rt := &recordingRT{body: okXML}
	client := NewClient(&ClientOptions{UserName: ncUserName, Transport: rt})
	var obj testResponse
	_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj)
	assert.NoError(t, err)
	assert.Equal(t, 1, rt.count())
}

// TestStressConcurrent hammers the client from many goroutines to surface data
// races (run under -race) and serialization regressions.
func TestStressConcurrent(t *testing.T) {
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
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var obj testResponse
			if _, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "x"}, &obj); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("unexpected error under concurrency: %v", err)
	}
}
