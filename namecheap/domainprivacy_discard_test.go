package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const domainPrivacyDiscardOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<CommandResponse Type="namecheap.whoisguard.discard">
			<WhoisguardDiscardResult IsSuccess="true" />
		</CommandResponse>
	</ApiResponse>
`

func TestDomainPrivacyService_Discard(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainPrivacyDiscardOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainPrivacy.DiscardWithContext(context.Background(), 53538)
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.whoisguard.discard", sentBody.Get("Command"))
		assert.Equal(t, "53538", sentBody.Get("WhoisguardID"))
		assert.True(t, *resp.Result.IsSuccess)
	})

	t.Run("invalid_id_no_http", func(t *testing.T) {
		t.Parallel()
		var called int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&called, 1)
			t.Errorf("server must not be called when validation fails")
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainPrivacy.DiscardWithContext(context.Background(), 0)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "WhoisguardID")
		}
		assert.Equal(t, int32(0), atomic.LoadInt32(&called))
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(apiErrorXML(2019166, "Domain not found")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainPrivacy.DiscardWithContext(context.Background(), 53538)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})

	// Discard is destructive and non-idempotent: an ambiguous server-side error
	// must not be retried, exactly like domains.create.
	t.Run("non_idempotent_no_retry_on_server_error", func(t *testing.T) {
		t.Parallel()
		var calls int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			_, _ = writer.Write([]byte(apiErrorXML(5050900, "Server exception")))
		}))
		defer mockServer.Close()

		client := newResilienceClient(mockServer.URL, func(o *ClientOptions) {
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.DomainPrivacy.DiscardWithContext(context.Background(), 53538)
		var apiErr *APIError
		assert.True(t, errors.As(err, &apiErr))
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a destructive discard must not be retried on an ambiguous server error")
		assert.NotContains(t, err.Error(), "after")
	})

	// The same no-retry guarantee for an ambiguous transport timeout.
	t.Run("non_idempotent_no_retry_on_transport_timeout", func(t *testing.T) {
		t.Parallel()
		rt := &countingErrRT{err: timeoutError{}}
		client := newResilienceClient("http://example.invalid", func(o *ClientOptions) {
			o.Transport = rt
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.DomainPrivacy.DiscardWithContext(context.Background(), 53538)
		assert.Error(t, err)
		assert.Equal(t, 1, rt.count(), "an ambiguous timeout must not be retried on a destructive discard")
	})
}
