package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func sslGetInfoOKResponse(status, expires string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.getInfo">
		<SSLGetInfoResult CertificateID="123456" Status="` + status + `" Type="PositiveSSL" CommonName="example.com" Provider="Comodo" IssuedOn="09/22/2020" Expires="` + expires + `" />
	</CommandResponse>
</ApiResponse>`
}

func TestSSLService_GetInfo(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_issued", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			// Expiry far in the future so IsExpiringSoon(30d) is false.
			_, _ = w.Write([]byte(sslGetInfoOKResponse("Active", "09/22/2099")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.GetInfoWithContext(context.Background(), 123456, "true", "Individual")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.getInfo", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("CertificateID"))
		assert.Equal(t, "true", sentBody.Get("Returncertificate"))
		assert.Equal(t, "Individual", sentBody.Get("Returntype"))

		result := resp.SSLGetInfoResult
		assert.Equal(t, 123456, *result.CertificateID)
		assert.Equal(t, "example.com", *result.CommonName)
		assert.Equal(t, CertStatusActive, result.CertStatus())
		assert.True(t, result.IsIssued())
		assert.False(t, result.IsExpiringSoon(30*24*time.Hour))
	})

	t.Run("expiring_soon_detected", func(t *testing.T) {
		t.Parallel()
		soon := time.Now().Add(5 * 24 * time.Hour).Format("01/02/2006")
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(sslGetInfoOKResponse("Active", soon)))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.GetInfoWithContext(context.Background(), 123456, "", "")
		assert.NoError(t, err)
		assert.True(t, resp.SSLGetInfoResult.IsExpiringSoon(30*24*time.Hour))
	})

	t.Run("invalid_id_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.GetInfoWithContext(context.Background(), 0, "", "")
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "CertificateID")
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

		_, err := client.SSL.GetInfoWithContext(context.Background(), 999, "", "")
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2033409, apiErr.Number)
		}
	})
}
