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

const domainPrivacyDisableOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<CommandResponse Type="namecheap.whoisguard.disable">
			<WhoisguardDisableResult DomainName="example.com" IsSuccess="true" />
		</CommandResponse>
	</ApiResponse>
`

func TestDomainPrivacyService_Disable(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainPrivacyDisableOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainPrivacy.DisableWithContext(context.Background(), 53536)
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.whoisguard.disable", sentBody.Get("Command"))
		assert.Equal(t, "53536", sentBody.Get("WhoisguardID"))
		assert.Equal(t, "example.com", *resp.Result.DomainName)
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

		_, err := client.DomainPrivacy.DisableWithContext(context.Background(), 0)
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

		_, err := client.DomainPrivacy.DisableWithContext(context.Background(), 53536)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})
}
