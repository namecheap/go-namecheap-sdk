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

	"github.com/stretchr/testify/assert"
)

const domainsSetRegistrarLockOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.setRegistrarLock</RequestedCommand>
		<CommandResponse Type="namecheap.domains.setRegistrarLock">
			<DomainSetRegistrarLockResult Domain="example.com" IsSuccess="true" />
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>0.5</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_SetRegistrarLock(t *testing.T) {
	t.Parallel()

	t.Run("lock_sends_LOCK", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsSetRegistrarLockOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.SetRegistrarLockWithContext(context.Background(), "example.com", RegistrarLock)
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		assert.Equal(t, "namecheap.domains.setRegistrarLock", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "LOCK", sentBody.Get("LockAction"))
		assert.Equal(t, true, *resp.DomainSetRegistrarLockResult.IsSuccess)
	})

	t.Run("unlock_sends_UNLOCK", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsSetRegistrarLockOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.SetRegistrarLockWithContext(context.Background(), "example.com", RegistrarUnlock)
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		assert.Equal(t, "UNLOCK", sentBody.Get("LockAction"))
	})

	t.Run("invalid_action_no_http", func(t *testing.T) {
		t.Parallel()
		var called int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&called, 1)
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.SetRegistrarLockWithContext(context.Background(), "example.com", LockAction("SIDEWAYS"))
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "LockAction")
		}
		assert.Equal(t, int32(0), atomic.LoadInt32(&called))
	})

	t.Run("empty_domain", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.SetRegistrarLockWithContext(context.Background(), "", RegistrarLock)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "DomainName")
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

		_, err := client.Domains.SetRegistrarLockWithContext(context.Background(), "notfound.com", RegistrarLock)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})
}
