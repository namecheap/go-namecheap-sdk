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

const sslCreateOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.create">
		<SSLCreateResult IsSuccess="true" OrderID="11223344" TransactionID="55667788" ChargedAmount="9.9800" CertificateID="123456" Created="09/22/2020" SSLType="PositiveSSL" />
	</CommandResponse>
</ApiResponse>`

func TestSSLService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslCreateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.CreateWithContext(context.Background(), &SSLCreateArgs{
			Years:         2,
			Type:          "PositiveSSL",
			SANStoADD:     3,
			PromotionCode: "PROMO",
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.create", sentBody.Get("Command"))
		assert.Equal(t, "2", sentBody.Get("Years"))
		assert.Equal(t, "PositiveSSL", sentBody.Get("Type"))
		assert.Equal(t, "3", sentBody.Get("SANStoADD"))
		assert.Equal(t, "PROMO", sentBody.Get("PromotionCode"))

		result := resp.SSLCreateResult
		assert.True(t, *result.IsSuccess)
		assert.Equal(t, 11223344, *result.OrderID)
		assert.Equal(t, Amount("9.9800"), *result.ChargedAmount)
		assert.Equal(t, 123456, *result.CertificateID)
	})

	t.Run("validation_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.CreateWithContext(context.Background(), &SSLCreateArgs{Years: 9})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Equal(t, []string{"Years", "Type"}, argErr.Fields)
		}

		_, err = client.SSL.CreateWithContext(context.Background(), nil)
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(2011170, "Promotion code invalid")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.CreateWithContext(context.Background(), &SSLCreateArgs{Years: 1, Type: "PositiveSSL"})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011170, apiErr.Number)
		}
	})

	// TestSSLService_Create non-idempotency: a retryable server error must NOT be
	// re-fired for the charge-bearing create call (exactly one attempt), proving
	// the doXML(..., false) classification.
	t.Run("non_idempotent_no_retry", func(t *testing.T) {
		t.Parallel()
		var calls int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			_, _ = w.Write([]byte(apiErrorXML(5050900, "Server exception")))
		}))
		defer mockServer.Close()

		client := newResilienceClient(mockServer.URL, func(o *ClientOptions) {
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.SSL.CreateWithContext(context.Background(), &SSLCreateArgs{Years: 1, Type: "PositiveSSL"})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 5050900, apiErr.Number)
		}
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a charge-bearing create must not be retried on an ambiguous server error")
	})
}
