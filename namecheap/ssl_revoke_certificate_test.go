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

const sslRevokeCertificateOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.revokecertificate">
		<SSLRevokeCertificateResult IsSuccess="true" CertificateID="123456" />
	</CommandResponse>
</ApiResponse>`

func TestSSLService_RevokeCertificate(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslRevokeCertificateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.RevokeCertificateWithContext(context.Background(), 123456, "PositiveSSL")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.revokecertificate", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("CertificateID"))
		assert.Equal(t, "PositiveSSL", sentBody.Get("CertificateType"))

		result := resp.SSLRevokeCertificateResult
		assert.True(t, *result.IsSuccess)
		assert.Equal(t, 123456, *result.CertificateID)
	})

	t.Run("validation_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.RevokeCertificateWithContext(context.Background(), 0, "")
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Equal(t, []string{"CertificateID", "CertificateType"}, argErr.Fields)
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

		_, err := client.SSL.RevokeCertificateWithContext(context.Background(), 999, "PositiveSSL")
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2033409, apiErr.Number)
		}
	})
}
