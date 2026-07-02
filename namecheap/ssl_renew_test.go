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

const sslRenewOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.renew">
		<SSLRenewResult CertificateID="123456" Years="1" OrderID="11223344" TransactionID="55667788" ChargedAmount="7.5000" />
	</CommandResponse>
</ApiResponse>`

func TestSSLService_Renew(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslRenewOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.RenewWithContext(context.Background(), &SSLRenewArgs{
			CertificateID: 123456,
			Years:         1,
			SSLType:       "PositiveSSL",
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.renew", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("CertificateID"))
		assert.Equal(t, "1", sentBody.Get("Years"))
		assert.Equal(t, "PositiveSSL", sentBody.Get("SSLType"))

		result := resp.SSLRenewResult
		assert.Equal(t, 123456, *result.CertificateID)
		assert.Equal(t, Amount("7.5000"), *result.ChargedAmount)
	})

	t.Run("validation_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.RenewWithContext(context.Background(), &SSLRenewArgs{})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Equal(t, []string{"CertificateID", "Years", "SSLType"}, argErr.Fields)
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(2033409, "Order not found")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.RenewWithContext(context.Background(), &SSLRenewArgs{CertificateID: 1, Years: 1, SSLType: "PositiveSSL"})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2033409, apiErr.Number)
		}
	})
}
