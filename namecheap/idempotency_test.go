package namecheap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// timeoutError is a net.Error whose Timeout reports true, so IsRetryable treats
// it as a retryable transport failure.
type timeoutError struct{}

func (timeoutError) Error() string   { return "simulated i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// countingErrRT is an http.RoundTripper that counts calls and always fails with
// a fixed error, exercising the transport-error retry path without a network.
type countingErrRT struct {
	err error

	mu    sync.Mutex
	calls int
}

func (rt *countingErrRT) RoundTrip(*http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.calls++
	rt.mu.Unlock()
	return nil, rt.err
}

func (rt *countingErrRT) count() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.calls
}

func retryingClient(baseURL string, transport http.RoundTripper) *Client {
	return newResilienceClient(baseURL, func(o *ClientOptions) {
		o.Transport = transport
		o.RateLimit = &RateLimitOptions{Disabled: true}
		o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
	})
}

// TestShouldRetryClassification unit-tests the idempotency-aware retry gate.
func TestShouldRetryClassification(t *testing.T) {
	t.Parallel()

	serverErr := &APIError{Number: 5050900, Message: "Server exception"}

	// 405 (pre-execution rate-limit) is always retryable.
	assert.True(t, shouldRetry(errRetryStatus, true))
	assert.True(t, shouldRetry(errRetryStatus, false))

	// A retryable server-side error: retried only for idempotent calls.
	assert.True(t, shouldRetry(serverErr, true))
	assert.False(t, shouldRetry(serverErr, false))

	// A transport timeout: retried only for idempotent calls.
	assert.True(t, shouldRetry(timeoutError{}, true))
	assert.False(t, shouldRetry(timeoutError{}, false))

	// A permanent error is never retried.
	assert.False(t, shouldRetry(&APIError{Number: 2019166}, true))
	assert.False(t, shouldRetry(&APIError{Number: 2019166}, false))
}

// TestDoXMLServerErrorRetryByClass proves the same retryable server-error XML is
// retried MaxAttempts times for an idempotent call but exactly once for a
// non-idempotent (charge-bearing) call.
func TestDoXMLServerErrorRetryByClass(t *testing.T) {
	t.Parallel()

	newServer := func(calls *int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(calls, 1)
			_, _ = w.Write([]byte(apiErrorXML(5050900, "Server exception")))
		}))
	}

	t.Run("idempotent_retries", func(t *testing.T) {
		t.Parallel()
		var calls int32
		server := newServer(&calls)
		defer server.Close()

		client := retryingClient(server.URL, nil)
		var obj testResponse
		_, err := client.doXML(context.Background(), map[string]string{"Command": "x"}, &obj, true)
		assert.Error(t, err)
		assert.Equal(t, int32(4), atomic.LoadInt32(&calls))
		assert.Contains(t, err.Error(), "after")
	})

	t.Run("non_idempotent_single_attempt", func(t *testing.T) {
		t.Parallel()
		var calls int32
		server := newServer(&calls)
		defer server.Close()

		client := retryingClient(server.URL, nil)
		var obj testResponse
		_, err := client.doXML(context.Background(), map[string]string{"Command": "x"}, &obj, false)
		var apiErr *APIError
		assert.True(t, errors.As(err, &apiErr))
		assert.Equal(t, 5050900, apiErr.Number)
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
		assert.NotContains(t, err.Error(), "after")
	})
}

// TestDoXMLTimeoutRetryByClass proves a transport timeout is retried for an
// idempotent call but not for a non-idempotent one.
func TestDoXMLTimeoutRetryByClass(t *testing.T) {
	t.Parallel()

	t.Run("idempotent_retries", func(t *testing.T) {
		t.Parallel()
		rt := &countingErrRT{err: timeoutError{}}
		client := retryingClient("", rt)
		var obj testResponse
		_, err := client.doXML(context.Background(), map[string]string{"Command": "x"}, &obj, true)
		assert.Error(t, err)
		assert.Equal(t, 4, rt.count())
	})

	t.Run("non_idempotent_single_attempt", func(t *testing.T) {
		t.Parallel()
		rt := &countingErrRT{err: timeoutError{}}
		client := retryingClient("", rt)
		var obj testResponse
		_, err := client.doXML(context.Background(), map[string]string{"Command": "x"}, &obj, false)
		assert.Error(t, err)
		assert.Equal(t, 1, rt.count(), "an ambiguous timeout must not be retried on a charge-bearing call")
	})
}

// TestDoXMLNonIdempotentRetriesOn405 proves the pre-execution HTTP 405
// rate-limit signal is still retried even for a non-idempotent call, because a
// 405 provably means the request was rejected before it executed.
func TestDoXMLNonIdempotentRetriesOn405(t *testing.T) {
	t.Parallel()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := retryingClient(server.URL, nil)
	var obj testResponse
	_, err := client.doXML(context.Background(), map[string]string{"Command": "x"}, &obj, false)
	assert.Error(t, err)
	assert.Greater(t, atomic.LoadInt32(&calls), int32(1), "405 is safe to resend and must be retried")
}
