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

const sslReissueOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.reissue">
		<SSLReissueResult ID="123456" IsSuccess="true">
			<HttpDCValidation ValueAvailable="true">
				<FileName>reissue.txt</FileName>
				<FileContent>hash</FileContent>
			</HttpDCValidation>
		</SSLReissueResult>
	</CommandResponse>
</ApiResponse>`

func TestSSLService_Reissue(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslReissueOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.ReissueWithContext(context.Background(), &SSLReissueArgs{
			CertificateID: 123456,
			CSR:           "csr-data",
			WebServerType: "nginx",
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.reissue", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("CertificateID"))
		assert.Equal(t, "csr-data", sentBody.Get("CSR"))
		assert.Equal(t, "nginx", sentBody.Get("WebServerType"))

		result := resp.SSLReissueResult
		assert.True(t, *result.IsSuccess)
		assert.Equal(t, "reissue.txt", *result.HTTPDCValidation.FileName)
	})

	t.Run("validation_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.ReissueWithContext(context.Background(), &SSLReissueArgs{})
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Equal(t, []string{"CertificateID", "CSR"}, argErr.Fields)
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(4011103, "Access denied")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.ReissueWithContext(context.Background(), &SSLReissueArgs{CertificateID: 1, CSR: "c"})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 4011103, apiErr.Number)
		}
	})

	// Reissue is classified non-idempotent: a retryable server error is not
	// re-fired (exactly one attempt).
	t.Run("non_idempotent_no_retry", func(t *testing.T) {
		t.Parallel()
		var calls int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			_, _ = w.Write([]byte(apiErrorXML(5050900, "Server exception")))
		}))
		defer mockServer.Close()

		client := newResilienceClient(mockServer.URL, func(o *ClientOptions) {
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, _ = client.SSL.ReissueWithContext(context.Background(), &SSLReissueArgs{CertificateID: 1, CSR: "c"})
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a reissue must not be retried on an ambiguous server error")
	})
}
