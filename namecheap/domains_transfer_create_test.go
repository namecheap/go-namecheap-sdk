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

const domainsTransferCreateOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.transfer.create</RequestedCommand>
		<CommandResponse Type="namecheap.domains.transfer.create">
			<DomainTransferCreateResult DomainName="example.com" TransferID="123456" StatusID="11" OrderID="987654" TransactionID="112233" ChargedAmount="10.87" />
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>1.234</ExecutionTime>
	</ApiResponse>
`

func TestDomainsTransferService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsTransferCreateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{
			DomainName:        "example.com",
			Years:             1,
			EPPCode:           "SECRET-EPP-CODE",
			PromotionCode:     "PROMO",
			AddFreeWhoisguard: Bool(true),
			WGenable:          Bool(false),
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.transfer.create", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "1", sentBody.Get("Years"))
		assert.Equal(t, "SECRET-EPP-CODE", sentBody.Get("EPPCode"))
		assert.Equal(t, "PROMO", sentBody.Get("PromotionCode"))
		assert.Equal(t, "Yes", sentBody.Get("AddFreeWhoisguard"))
		assert.Equal(t, "No", sentBody.Get("WGenable"))

		result := resp.DomainTransferCreateResult
		assert.Equal(t, "example.com", *result.DomainName)
		assert.Equal(t, 123456, *result.TransferID)
		assert.Equal(t, 11, *result.StatusID)
		assert.Equal(t, 987654, *result.OrderID)
		assert.Equal(t, 112233, *result.TransactionID)
	})

	t.Run("money_preserved_exactly_as_string", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(domainsTransferCreateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{
			DomainName: "example.com",
			Years:      1,
			EPPCode:    "abc",
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		// 10.87 cannot be represented exactly in binary floating point; the Amount
		// type keeps the exact server string.
		assert.Equal(t, Amount("10.87"), *resp.DomainTransferCreateResult.ChargedAmount)
		assert.Equal(t, "10.87", resp.DomainTransferCreateResult.ChargedAmount.String())
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.DomainsTransfer.CreateWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("missing_required_fields_reported_all_at_once_no_http", func(t *testing.T) {
		t.Parallel()
		var called int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&called, 1)
			t.Errorf("server must not be called when validation fails")
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "DomainName")
			assert.Contains(t, argErr.Fields, "Years")
			assert.Contains(t, argErr.Fields, "EPPCode")
		}
		assert.Equal(t, int32(0), atomic.LoadInt32(&called), "no charge-bearing call may happen when validation fails")
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(apiErrorXML(2011170, "Promotion code is invalid")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{
			DomainName: "example.com",
			Years:      1,
			EPPCode:    "abc",
		})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011170, apiErr.Number)
		}
	})

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

		_, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{
			DomainName: "example.com",
			Years:      1,
			EPPCode:    "abc",
		})
		var apiErr *APIError
		assert.True(t, errors.As(err, &apiErr))
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a charge-bearing transfer must not be retried on an ambiguous server error")
		assert.NotContains(t, err.Error(), "after")
	})

	t.Run("non_idempotent_no_retry_on_transport_timeout", func(t *testing.T) {
		t.Parallel()
		rt := &countingErrRT{err: timeoutError{}}
		client := newResilienceClient("http://example.invalid", func(o *ClientOptions) {
			o.Transport = rt
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{
			DomainName: "example.com",
			Years:      1,
			EPPCode:    "abc",
		})
		assert.Error(t, err)
		assert.Equal(t, 1, rt.count(), "an ambiguous timeout must not be retried on a charge-bearing transfer")
	})
}
