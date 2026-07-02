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

const sslGetListOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.getList">
		<SSLListResult>
			<SSL CertificateID="123456" HostName="example.com" SSLType="PositiveSSL" PurchaseDate="09/22/2020" ExpireDate="09/22/2021" Status="Active" />
			<SSL CertificateID="123457" HostName="foo.example" SSLType="EV SSL" PurchaseDate="01/01/2021" ExpireDate="01/01/2022" Status="Purchased" />
		</SSLListResult>
		<Paging>
			<TotalItems>2</TotalItems>
			<CurrentPage>1</CurrentPage>
			<PageSize>20</PageSize>
		</Paging>
	</CommandResponse>
</ApiResponse>`

func TestSSLService_GetList(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_filters", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslGetListOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.SSL.GetListWithContext(context.Background(), &SSLGetListArgs{
			ListType:   String("Active"),
			SearchTerm: String("example"),
			Page:       Int(2),
			PageSize:   Int(50),
			SortBy:     String("EXPIREDATETIME"),
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.ssl.getList", sentBody.Get("Command"))
		assert.Equal(t, "Active", sentBody.Get("ListType"))
		assert.Equal(t, "example", sentBody.Get("SearchTerm"))
		assert.Equal(t, "2", sentBody.Get("Page"))
		assert.Equal(t, "50", sentBody.Get("PageSize"))
		assert.Equal(t, "EXPIREDATETIME", sentBody.Get("SortBy"))

		certs := *resp.SSLCertificates
		assert.Len(t, certs, 2)
		assert.Equal(t, 123456, *certs[0].CertificateID)
		assert.Equal(t, "example.com", *certs[0].HostName)
		assert.Equal(t, "PositiveSSL", *certs[0].SSLType)
		assert.Equal(t, "Active", *certs[0].Status)
		assert.Equal(t, 2, *resp.Paging.TotalItems)
	})

	t.Run("nil_args_ok", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = w.Write([]byte(sslGetListOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.GetListWithContext(context.Background(), nil)
		assert.NoError(t, err)
		assert.Equal(t, "namecheap.ssl.getList", sentBody.Get("Command"))
		assert.Empty(t, sentBody.Get("ListType"))
	})

	t.Run("invalid_filters_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		for _, args := range []*SSLGetListArgs{
			{ListType: String("Bogus")},
			{SortBy: String("Bogus")},
			{Page: Int(0)},
			{PageSize: Int(5)},
			{PageSize: Int(500)},
		} {
			_, err := client.SSL.GetListWithContext(context.Background(), args)
			var argErr *InvalidArgumentsError
			assert.True(t, errors.As(err, &argErr))
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(apiErrorXML(2011170, "Promotion code invalid")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.SSL.GetListWithContext(context.Background(), nil)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011170, apiErr.Number)
		}
	})
}
