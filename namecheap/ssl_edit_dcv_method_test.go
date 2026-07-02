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

const sslEditDCVMethodOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.editDCVMethod">
		<SSLEditDCVMethodResult ID="123456" IsSuccess="true">
			<HttpDCValidation ValueAvailable="true">
				<FileName>ABCDEF.txt</FileName>
				<FileContent>hash-content</FileContent>
			</HttpDCValidation>
		</SSLEditDCVMethodResult>
	</CommandResponse>
</ApiResponse>`

func TestSSLService_EditDCVMethod(t *testing.T) {
	t.Parallel()

	t.Run("single_domain_success", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslEditDCVMethodOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.EditDCVMethodWithContext(context.Background(), &SSLEditDCVMethodArgs{
			CertificateID: 123456,
			DCVMethod:     DCVMethodDNS,
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.editDCVMethod", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("CertificateID"))
		assert.Equal(t, "CNAME_CSR_HASH", sentBody.Get("DCVMethod"))
		assert.Empty(t, sentBody.Get("DNSNames"))

		result := resp.SSLEditDCVMethodResult
		assert.Equal(t, 123456, *result.ID)
		assert.True(t, *result.IsSuccess)
		assert.True(t, *result.HTTPDCValidation.ValueAvailable)
		assert.Equal(t, "ABCDEF.txt", *result.HTTPDCValidation.FileName)
	})

	t.Run("multi_domain_comma_lists", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslEditDCVMethodOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.EditDCVMethodWithContext(context.Background(), &SSLEditDCVMethodArgs{
			CertificateID: 123456,
			SANs: []SANEntry{
				{DomainName: "a.com", DCVMethod: DCVMethodHTTP},
				{DomainName: "b.com", DCVMethod: DCVMethodDNS},
				{DomainName: "c.com", DCVMethod: DCVMethodEmail, ApproverEmail: "[email protected]"},
			},
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "a.com,b.com,c.com", sentBody.Get("DNSNames"))
		assert.Equal(t, "HTTP_CSR_HASH,CNAME_CSR_HASH,[email protected]", sentBody.Get("DCVMethods"))
		assert.Empty(t, sentBody.Get("DCVMethod"))
	})

	t.Run("validation_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		cases := []struct {
			name string
			args *SSLEditDCVMethodArgs
			want []string
		}{
			{"missing_cert_and_method", &SSLEditDCVMethodArgs{}, []string{"CertificateID", "DCVMethod"}},
			{"missing_method_only", &SSLEditDCVMethodArgs{CertificateID: 1}, []string{"DCVMethod"}},
			{"email_single_missing_approver", &SSLEditDCVMethodArgs{CertificateID: 1, DCVMethod: DCVMethodEmail}, []string{"ApproverEmail"}},
			{"san_email_missing_approver", &SSLEditDCVMethodArgs{CertificateID: 1, SANs: []SANEntry{{DomainName: "x.com", DCVMethod: DCVMethodEmail}}}, []string{"SANs[0].ApproverEmail"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := client.SSL.EditDCVMethodWithContext(context.Background(), tc.args)
				var argErr *InvalidArgumentsError
				if assert.True(t, errors.As(err, &argErr)) {
					assert.Equal(t, tc.want, argErr.Fields)
				}
			})
		}
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"
		_, err := client.SSL.EditDCVMethodWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(2030166, "Domain invalid")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.EditDCVMethodWithContext(context.Background(), &SSLEditDCVMethodArgs{CertificateID: 1, DCVMethod: DCVMethodHTTP})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2030166, apiErr.Number)
		}
	})
}
