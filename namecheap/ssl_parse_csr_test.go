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

const sslParseCSROKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.parseCSR">
		<SSLParseCSRResult>
			<CSRDetails>
				<CommonName>example.com</CommonName>
				<DomainName>example.com</DomainName>
				<Country>US</Country>
				<OrganisationUnit>IT</OrganisationUnit>
				<Organisation>Example Inc</Organisation>
				<State>CA</State>
				<Locality>San Francisco</Locality>
				<Email>[email protected]</Email>
			</CSRDetails>
		</SSLParseCSRResult>
	</CommandResponse>
</ApiResponse>`

func TestSSLService_ParseCSR(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslParseCSROKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.ParseCSRWithContext(context.Background(), "-----BEGIN CERTIFICATE REQUEST-----\nMIIC...\n-----END CERTIFICATE REQUEST-----", "PositiveSSL")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.parseCSR", sentBody.Get("Command"))
		assert.Contains(t, sentBody.Get("csr"), "BEGIN CERTIFICATE REQUEST")
		assert.Equal(t, "PositiveSSL", sentBody.Get("CertificateType"))

		result := resp.SSLParseCSRResult
		assert.Equal(t, "example.com", *result.CommonName)
		assert.Equal(t, "US", *result.Country)
		assert.Equal(t, "Example Inc", *result.Organisation)
		assert.Equal(t, "[email protected]", *result.Email)
	})

	t.Run("missing_csr_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.ParseCSRWithContext(context.Background(), "", "")
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "csr")
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(2030166, "Domain invalid")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.ParseCSRWithContext(context.Background(), "csr-data", "")
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2030166, apiErr.Number)
		}
	})
}
