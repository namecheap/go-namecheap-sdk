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

const domainsRenewOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.renew</RequestedCommand>
		<CommandResponse Type="namecheap.domains.renew">
			<DomainRenewResult DomainName="example.com" DomainID="1234567" Renew="true" OrderID="987654" TransactionID="112233" ChargedAmount="9.56">
				<DomainDetails>
					<ExpiredDate>10/13/2027</ExpiredDate>
					<NumYears>0</NumYears>
				</DomainDetails>
			</DomainRenewResult>
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>1.234</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_Renew(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsRenewOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.RenewWithContext(context.Background(), &DomainsRenewArgs{
			DomainName:    "example.com",
			Years:         1,
			PromotionCode: "PROMO",
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.renew", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "1", sentBody.Get("Years"))
		assert.Equal(t, "PROMO", sentBody.Get("PromotionCode"))

		result := resp.DomainRenewResult
		assert.Equal(t, "example.com", *result.DomainName)
		assert.Equal(t, 1234567, *result.DomainID)
		assert.Equal(t, true, *result.Renew)
		assert.Equal(t, 987654, *result.OrderID)
		assert.Equal(t, 112233, *result.TransactionID)
		assert.Equal(t, Amount("9.56"), *result.ChargedAmount)
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.RenewWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("missing_required_fields", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.RenewWithContext(context.Background(), &DomainsRenewArgs{})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "DomainName")
			assert.Contains(t, argErr.Fields, "Years")
		}
	})

	t.Run("premium_guard_blocks_missing_price", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.RenewWithContext(context.Background(), &DomainsRenewArgs{
			DomainName:      "example.com",
			Years:           1,
			IsPremiumDomain: true,
		})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "PremiumPrice")
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="2019166">Domain not found</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.RenewWithContext(context.Background(), &DomainsRenewArgs{DomainName: "notfound.com", Years: 1})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
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

		_, err := client.Domains.RenewWithContext(context.Background(), &DomainsRenewArgs{DomainName: "example.com", Years: 1})
		assert.Error(t, err)
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a charge-bearing renew must not be retried on an ambiguous server error")
	})
}
