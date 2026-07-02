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

const domainPrivacyGetListPage1Response = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<RequestedCommand>namecheap.whoisguard.getlist</RequestedCommand>
		<CommandResponse Type="namecheap.whoisguard.getlist">
			<WhoisguardGetListResult>
				<Whoisguard ID="53536" DomainName="alpha.com" Created="10/22/2023" Expires="10/22/2024" Status="ENABLED" />
				<Whoisguard ID="53537" DomainName="beta.net" Created="11/01/2023" Expires="11/01/2024" Status="DISABLED" />
				<Whoisguard ID="53538" DomainName="" Created="12/01/2023" Expires="12/01/2024" Status="FREE" />
			</WhoisguardGetListResult>
			<Paging>
				<TotalItems>5</TotalItems>
				<CurrentPage>1</CurrentPage>
				<PageSize>3</PageSize>
			</Paging>
		</CommandResponse>
	</ApiResponse>
`

const domainPrivacyGetListEmptyResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<RequestedCommand>namecheap.whoisguard.getlist</RequestedCommand>
		<CommandResponse Type="namecheap.whoisguard.getlist">
			<WhoisguardGetListResult />
			<Paging>
				<TotalItems>0</TotalItems>
				<CurrentPage>1</CurrentPage>
				<PageSize>20</PageSize>
			</Paging>
		</CommandResponse>
	</ApiResponse>
`

func TestDomainPrivacyService_GetList(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_paging", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainPrivacyGetListPage1Response))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainPrivacy.GetListWithContext(context.Background(), &DomainPrivacyGetListArgs{
			ListType: String("ALL"),
			Page:     Int(1),
			PageSize: Int(3),
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.whoisguard.getlist", sentBody.Get("Command"))
		assert.Equal(t, "ALL", sentBody.Get("ListType"))
		assert.Equal(t, "1", sentBody.Get("Page"))
		assert.Equal(t, "3", sentBody.Get("PageSize"))

		entries := *resp.DomainPrivacyList
		assert.Len(t, entries, 3)

		// IDs are typed ints, never strings.
		assert.Equal(t, 53536, *entries[0].ID)
		assert.Equal(t, "alpha.com", *entries[0].DomainName)
		assert.Equal(t, "10/22/2023", *entries[0].Created)
		assert.Equal(t, "10/22/2024", *entries[0].Expires)
		assert.Equal(t, "ENABLED", *entries[0].Status)
		assert.Equal(t, PrivacyStateAllotted, entries[0].State())
		assert.True(t, entries[0].IsEnabled())

		assert.Equal(t, PrivacyStateAllotted, entries[1].State())
		assert.False(t, entries[1].IsEnabled())

		assert.Equal(t, 53538, *entries[2].ID)
		assert.Equal(t, PrivacyStateFree, entries[2].State())

		assert.Equal(t, 5, *resp.Paging.TotalItems)
		assert.Equal(t, 1, *resp.Paging.CurrentPage)
		assert.Equal(t, 3, *resp.Paging.PageSize)
	})

	t.Run("empty_list", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(domainPrivacyGetListEmptyResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainPrivacy.GetListWithContext(context.Background(), &DomainPrivacyGetListArgs{ListType: String("FREE")})
		assert.NoError(t, err)
		// An empty <WhoisguardGetListResult/> yields a nil (or empty) list, not an error.
		if resp.DomainPrivacyList != nil {
			assert.Empty(t, *resp.DomainPrivacyList)
		}
		assert.Equal(t, 0, *resp.Paging.TotalItems)
	})

	t.Run("multi_page_paging_reflected", func(t *testing.T) {
		t.Parallel()
		// Page 2 fixture: paging block echoes the requested page, proving paging
		// round-trips independently of the list contents.
		page2 := `<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<CommandResponse Type="namecheap.whoisguard.getlist">
					<WhoisguardGetListResult>
						<Whoisguard ID="60001" DomainName="gamma.io" Created="01/02/2024" Expires="01/02/2025" Status="ENABLED" />
						<Whoisguard ID="60002" DomainName="delta.dev" Created="01/03/2024" Expires="01/03/2025" Status="DISABLED" />
					</WhoisguardGetListResult>
					<Paging>
						<TotalItems>5</TotalItems>
						<CurrentPage>2</CurrentPage>
						<PageSize>3</PageSize>
					</Paging>
				</CommandResponse>
			</ApiResponse>`

		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(page2))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainPrivacy.GetListWithContext(context.Background(), &DomainPrivacyGetListArgs{Page: Int(2), PageSize: Int(3)})
		assert.NoError(t, err)
		assert.Equal(t, "2", sentBody.Get("Page"))
		assert.Len(t, *resp.DomainPrivacyList, 2)
		assert.Equal(t, 2, *resp.Paging.CurrentPage)
		assert.Equal(t, 5, *resp.Paging.TotalItems)
	})

	t.Run("nil_args_sends_only_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainPrivacyGetListEmptyResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainPrivacy.GetListWithContext(context.Background(), nil)
		assert.NoError(t, err)
		assert.Equal(t, "namecheap.whoisguard.getlist", sentBody.Get("Command"))
		assert.Empty(t, sentBody.Get("ListType"))
		assert.Empty(t, sentBody.Get("Page"))
	})

	t.Run("invalid_list_type_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainPrivacy.GetListWithContext(context.Background(), &DomainPrivacyGetListArgs{ListType: String("BOGUS")})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ListType")
	})

	t.Run("invalid_page_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainPrivacy.GetListWithContext(context.Background(), &DomainPrivacyGetListArgs{Page: Int(0)})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Page")
	})

	t.Run("invalid_page_size_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainPrivacy.GetListWithContext(context.Background(), &DomainPrivacyGetListArgs{PageSize: Int(1)})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PageSize")
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(apiErrorXML(2011170, "Some error")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainPrivacy.GetListWithContext(context.Background(), nil)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011170, apiErr.Number)
		}
	})
}
