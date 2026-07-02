package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const domainsReactivateOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.reactivate</RequestedCommand>
		<CommandResponse Type="namecheap.domains.reactivate">
			<DomainReactivateResult Domain="example.com" IsSuccess="true" ChargedAmount="650.78" OrderID="23628" TransactionID="24581" />
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>1.234</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_Reactivate(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsReactivateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.ReactivateWithContext(context.Background(), &DomainsReactivateArgs{
			DomainName: "example.com",
			YearsToAdd: 1,
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.reactivate", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "1", sentBody.Get("YearsToAdd"))

		result := resp.DomainReactivateResult
		assert.Equal(t, "example.com", *result.Domain)
		assert.Equal(t, true, *result.IsSuccess)
		assert.Equal(t, Amount("650.78"), *result.ChargedAmount)
		assert.Equal(t, 23628, *result.OrderID)
		assert.Equal(t, 24581, *result.TransactionID)
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.ReactivateWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("missing_domain", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.ReactivateWithContext(context.Background(), &DomainsReactivateArgs{})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "DomainName")
		}
	})

	t.Run("premium_guard_blocks_missing_price", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.ReactivateWithContext(context.Background(), &DomainsReactivateArgs{
			DomainName:      "example.com",
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

		_, err := client.Domains.ReactivateWithContext(context.Background(), &DomainsReactivateArgs{DomainName: "notfound.com"})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})
}
