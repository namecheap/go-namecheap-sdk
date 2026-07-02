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

const domainsGetRegistrarLockOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.getRegistrarLock</RequestedCommand>
		<CommandResponse Type="namecheap.domains.getRegistrarLock">
			<DomainGetRegistrarLockResult Domain="example.com" RegistrarLockStatus="true" />
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>0.5</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_GetRegistrarLock(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsGetRegistrarLockOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetRegistrarLockWithContext(context.Background(), "example.com")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.getRegistrarLock", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "example.com", *resp.DomainGetRegistrarLockResult.Domain)
		assert.Equal(t, true, *resp.DomainGetRegistrarLockResult.RegistrarLockStatus)
	})

	t.Run("empty_domain", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.GetRegistrarLockWithContext(context.Background(), "")
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
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

		_, err := client.Domains.GetRegistrarLockWithContext(context.Background(), "notfound.com")
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})
}
