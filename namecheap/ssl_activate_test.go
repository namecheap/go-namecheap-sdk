package namecheap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const sslActivateOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.activate">
		<SSLActivateResult ID="123456" IsSuccess="true">
			<HttpDCValidation ValueAvailable="true">
				<FileName>A1B2C3.txt</FileName>
				<FileContent>hash-content</FileContent>
			</HttpDCValidation>
		</SSLActivateResult>
	</CommandResponse>
</ApiResponse>`

func newSSLCaptureServer(t *testing.T, body string) (*httptest.Server, *url.Values) {
	t.Helper()
	sent := &url.Values{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		parsed, _ := url.ParseQuery(string(raw))
		*sent = parsed
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, sent
}

// TestSSLService_Activate_PerDCVMethod runs one activation per DCV method and
// asserts the DCV method serializes to the correct wire value, plus a successful
// response parse (including the HTTP DCV file block).
func TestSSLService_Activate_PerDCVMethod(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		method   DCVMethod
		approver string
		wantWire string
	}{
		{"http", DCVMethodHTTP, "", "HTTP_CSR_HASH"},
		{"dns", DCVMethodDNS, "", "CNAME_CSR_HASH"},
		{"email", DCVMethodEmail, "[email protected]", "[email protected]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, sent := newSSLCaptureServer(t, sslActivateOKResponse)
			client := setupClient(nil)
			client.BaseURL = server.URL

			resp, err := client.SSL.ActivateWithContext(context.Background(), &SSLActivateArgs{
				CertificateID:     123456,
				CSR:               "csr-data",
				AdminEmailAddress: "[email protected]",
				WebServerType:     "nginx",
				DCVMethod:         tc.method,
				ApproverEmail:     tc.approver,
			})
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			assert.Equal(t, "namecheap.ssl.activate", sent.Get("Command"))
			assert.Equal(t, "123456", sent.Get("CertificateID"))
			assert.Equal(t, "csr-data", sent.Get("CSR"))
			assert.Equal(t, "[email protected]", sent.Get("AdminEmailAddress"))
			assert.Equal(t, "nginx", sent.Get("WebServerType"))
			assert.Equal(t, tc.wantWire, sent.Get("DCVMethod"))

			result := resp.SSLActivateResult
			assert.Equal(t, 123456, *result.ID)
			assert.True(t, *result.IsSuccess)
			assert.Equal(t, "A1B2C3.txt", *result.HTTPDCValidationFileName)
			assert.Equal(t, "hash-content", *result.HTTPDCValidationFileContent)
		})
	}
}

// TestSSLService_Activate_MultiSAN round-trips 3 SAN host blocks through the
// indexed serialization and back out of the captured request.
func TestSSLService_Activate_MultiSAN(t *testing.T) {
	t.Parallel()

	server, sent := newSSLCaptureServer(t, sslActivateOKResponse)
	client := setupClient(nil)
	client.BaseURL = server.URL

	sans := []SANEntry{
		{DomainName: "a.example.com", DCVMethod: DCVMethodHTTP},
		{DomainName: "b.example.com", DCVMethod: DCVMethodDNS},
		{DomainName: "c.example.com", DCVMethod: DCVMethodEmail, ApproverEmail: "[email protected]"},
	}

	_, err := client.SSL.ActivateWithContext(context.Background(), &SSLActivateArgs{
		CertificateID:     123456,
		CSR:               "csr-data",
		AdminEmailAddress: "[email protected]",
		DCVMethod:         DCVMethodHTTP,
		SANs:              sans,
	})
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	wantNames := []string{"a.example.com", "b.example.com", "c.example.com"}
	wantMethods := []string{"HTTP_CSR_HASH", "CNAME_CSR_HASH", "[email protected]"}
	for i := range sans {
		assert.Equal(t, wantNames[i], sent.Get(fmt.Sprintf("SANDomainName[%d]", i)))
		assert.Equal(t, wantMethods[i], sent.Get(fmt.Sprintf("SANDCVMethod[%d]", i)))
	}
}

// TestSSLService_Activate_Validation table-tests that every missing required and
// per-DCV-method field is reported at once, before any HTTP call.
func TestSSLService_Activate_Validation(t *testing.T) {
	t.Parallel()

	client := setupClient(nil)
	client.BaseURL = "http://127.0.0.1:0"

	cases := []struct {
		name string
		args *SSLActivateArgs
		want []string
	}{
		{
			"all_required_missing",
			&SSLActivateArgs{},
			[]string{"CertificateID", "CSR", "AdminEmailAddress"},
		},
		{
			"email_dcv_missing_approver",
			&SSLActivateArgs{CertificateID: 1, CSR: "c", AdminEmailAddress: "[email protected]", DCVMethod: DCVMethodEmail},
			[]string{"ApproverEmail"},
		},
		{
			"san_missing_fields",
			&SSLActivateArgs{
				CertificateID: 1, CSR: "c", AdminEmailAddress: "[email protected]",
				SANs: []SANEntry{{DCVMethod: DCVMethodEmail}},
			},
			[]string{"SANs[0].DomainName", "SANs[0].ApproverEmail"},
		},
		{
			"san_missing_method",
			&SSLActivateArgs{
				CertificateID: 1, CSR: "c", AdminEmailAddress: "[email protected]",
				SANs: []SANEntry{{DomainName: "x.com"}},
			},
			[]string{"SANs[0].DCVMethod"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := client.SSL.ActivateWithContext(context.Background(), tc.args)
			var argErr *InvalidArgumentsError
			if assert.True(t, errors.As(err, &argErr)) {
				assert.Equal(t, tc.want, argErr.Fields)
			}
		})
	}

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		_, err := client.SSL.ActivateWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})
}

func TestSSLService_Activate_APIError(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(apiErrorXML(4011103, "Access denied")))
	}))
	defer mockServer.Close()

	client := setupClient(nil)
	client.BaseURL = mockServer.URL

	_, err := client.SSL.ActivateWithContext(context.Background(), &SSLActivateArgs{
		CertificateID: 1, CSR: "c", AdminEmailAddress: "[email protected]",
	})
	var apiErr *APIError
	if assert.True(t, errors.As(err, &apiErr)) {
		assert.Equal(t, 4011103, apiErr.Number)
	}
}
