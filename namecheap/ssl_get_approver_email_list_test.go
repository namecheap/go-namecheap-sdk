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

const sslGetApproverEmailListOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.getApproverEmailList">
		<GetApproverEmailListResult>
			<Domainemails>
				<email>[email protected]</email>
			</Domainemails>
			<Genericemails>
				<email>[email protected]</email>
				<email>[email protected]</email>
			</Genericemails>
			<Manualemails>
				<email>[email protected]</email>
			</Manualemails>
		</GetApproverEmailListResult>
	</CommandResponse>
</ApiResponse>`

func TestSSLService_GetApproverEmailList(t *testing.T) {
	t.Parallel()

	t.Run("success_parse", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslGetApproverEmailListOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.GetApproverEmailListWithContext(context.Background(), "example.com", "PositiveSSL")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.getApproverEmailList", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "PositiveSSL", sentBody.Get("CertificateType"))

		assert.Equal(t, []string{"[email protected]"}, resp.DomainEmails)
		assert.Equal(t, []string{"[email protected]", "[email protected]"}, resp.GenericEmails)
		assert.Equal(t, []string{"[email protected]"}, resp.ManualEmails)
	})

	t.Run("missing_fields_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.SSL.GetApproverEmailListWithContext(context.Background(), "", "")
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Equal(t, []string{"DomainName", "CertificateType"}, argErr.Fields)
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(2019166, "Domain not found")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.GetApproverEmailListWithContext(context.Background(), "example.com", "PositiveSSL")
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})
}
