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

const sslPurchaseMoreSansOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.purchasemoresans">
		<SSLPurchaseMoreSansResult IsSuccess="true" OrderID="11223344" TransactionID="55667788" ChargedAmount="15.0000" CertificateID="123456" SSLType="PositiveSSL Multi-Domain" SANSCount="5" />
	</CommandResponse>
</ApiResponse>`

func TestSSLService_PurchaseMoreSans(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslPurchaseMoreSansOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.PurchaseMoreSansWithContext(context.Background(), &SSLPurchaseMoreSansArgs{
			CertificateID:     123456,
			NumberOfSANSToAdd: 2,
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.purchasemoresans", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("CertificateID"))
		assert.Equal(t, "2", sentBody.Get("NumberOfSANSToAdd"))

		result := resp.SSLPurchaseMoreSansResult
		assert.True(t, *result.IsSuccess)
		assert.Equal(t, Amount("15.0000"), *result.ChargedAmount)
		assert.Equal(t, 5, *result.SANSCount)
	})

	t.Run("validation_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.PurchaseMoreSansWithContext(context.Background(), &SSLPurchaseMoreSansArgs{})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Equal(t, []string{"CertificateID", "NumberOfSANSToAdd"}, argErr.Fields)
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

		_, err := client.SSL.PurchaseMoreSansWithContext(context.Background(), &SSLPurchaseMoreSansArgs{CertificateID: 1, NumberOfSANSToAdd: 1})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2033409, apiErr.Number)
		}
	})
}
