package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDomainsGetList(t *testing.T) {
	t.Parallel()
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.getlist</RequestedCommand>
			<CommandResponse Type="namecheap.domains.getList">
				<DomainGetListResult>
					<Domain ID="677625" Name="domain.com" User="user" Created="06/02/2021" Expires="06/02/2022" IsExpired="false" IsLocked="false" AutoRenew="true" WhoisGuard="ENABLED" IsPremium="false" IsOurDNS="false" />
					<Domain ID="677626" Name="domain2.net" User="user" Created="06/02/2021" Expires="06/02/2022" IsExpired="false" IsLocked="false" AutoRenew="true" WhoisGuard="ENABLED" IsPremium="false" IsOurDNS="true" />
				</DomainGetListResult>
				<Paging>
					<TotalItems>2</TotalItems>
					<CurrentPage>1</CurrentPage>
					<PageSize>20</PageSize>
				</Paging>
			</CommandResponse>
			<Server>PHX01SBAPIEXT05</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.011</ExecutionTime>
		</ApiResponse>
	`

	t.Run("request_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(&DomainsGetListArgs{})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.getList", sentBody.Get("Command"))
	})

	t.Run("request_data_passing", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(&DomainsGetListArgs{
			ListType:   String("EXPIRING"),
			SearchTerm: String("search.com"),
			Page:       Int(2),
			PageSize:   Int(10),
			SortBy:     String("EXPIREDATE_DESC"),
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "EXPIRING", sentBody.Get("ListType"))
		assert.Equal(t, "search.com", sentBody.Get("SearchTerm"))
		assert.Equal(t, "2", sentBody.Get("Page"))
		assert.Equal(t, "10", sentBody.Get("PageSize"))
		assert.Equal(t, "EXPIREDATE_DESC", sentBody.Get("SortBy"))
	})

	t.Run("request_data_when_nil_input_arguments", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(nil)
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Empty(t, sentBody.Get("Page"))
		assert.Empty(t, sentBody.Get("PageSize"))
		assert.Empty(t, sentBody.Get("SortBy"))
		assert.Empty(t, sentBody.Get("SearchTerm"))
		assert.Empty(t, sentBody.Get("ListType"))
	})

	t.Run("request_data_page_error", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(&DomainsGetListArgs{
			Page: Int(-1),
		})

		assert.EqualError(t, err, "invalid Page value: -1, minimum value is 1")
	})

	t.Run("request_data_page_size_too_low_error", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(&DomainsGetListArgs{
			PageSize: Int(3),
		})

		assert.EqualError(t, err, "invalid PageSize value: 3, minimum value is 10, and maximum value is 100")
	})

	t.Run("request_data_page_size_too_high_error", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(&DomainsGetListArgs{
			PageSize: Int(999),
		})

		assert.EqualError(t, err, "invalid PageSize value: 999, minimum value is 10, and maximum value is 100")
	})

	t.Run("request_data_invalid_sort_by_error", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)

		_, err := client.Domains.GetList(&DomainsGetListArgs{
			SortBy: String("INVALID"),
		})

		assert.EqualError(t, err, "invalid SortBy value: INVALID")
	})

	t.Run("request_data_invalid_list_type_error", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)

		_, err := client.Domains.GetList(&DomainsGetListArgs{
			ListType: String("BADTYPE"),
		})

		assert.EqualError(t, err, "invalid ListType value: BADTYPE")
	})

	t.Run("correct_parsing_domain_list", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		response, err := client.Domains.GetList(&DomainsGetListArgs{})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		expiresDate, _ := time.Parse("01/02/2006", "06/02/2022")
		createdDate, _ := time.Parse("01/02/2006", "06/02/2021")

		expectedDomains := []Domain{
			{
				ID:         String("677625"),
				Name:       String("domain.com"),
				User:       String("user"),
				Created:    &DateTime{createdDate},
				Expires:    &DateTime{expiresDate},
				IsExpired:  Bool(false),
				IsLocked:   Bool(false),
				AutoRenew:  Bool(true),
				WhoisGuard: String("ENABLED"),
				IsPremium:  Bool(false),
				IsOurDNS:   Bool(false),
			},
			{
				ID:         String("677626"),
				Name:       String("domain2.net"),
				User:       String("user"),
				Created:    &DateTime{createdDate},
				Expires:    &DateTime{expiresDate},
				IsExpired:  Bool(false),
				IsLocked:   Bool(false),
				AutoRenew:  Bool(true),
				WhoisGuard: String("ENABLED"),
				IsPremium:  Bool(false),
				IsOurDNS:   Bool(true),
			},
		}

		assert.Equal(t, &expectedDomains, response.Domains)
	})

	t.Run("correct_parsing_paging", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		response, err := client.Domains.GetList(&DomainsGetListArgs{})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		expectedPaging := DomainsGetListPaging{
			TotalItems:  Int(2),
			CurrentPage: Int(1),
			PageSize:    Int(20),
		}

		assert.Equal(t, &expectedPaging, response.Paging)
	})

	t.Run("empty_domain_list", func(t *testing.T) {
		t.Parallel()
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<Warnings />
				<RequestedCommand>namecheap.domains.getlist</RequestedCommand>
				<CommandResponse Type="namecheap.domains.getList">
					<DomainGetListResult></DomainGetListResult>
					<Paging>
						<TotalItems>2</TotalItems>
						<CurrentPage>1</CurrentPage>
						<PageSize>20</PageSize>
					</Paging>
				</CommandResponse>
				<Server>PHX01SBAPIEXT05</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.011</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		response, err := client.Domains.GetList(&DomainsGetListArgs{})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Nil(t, response.Domains)
	})

	t.Run("server_empty_response", func(t *testing.T) {
		t.Parallel()
		fakeLocalResponse := ""

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(nil)

		assert.EqualError(t, err, "unable to parse server response: EOF")
	})

	t.Run("server_non_xml_response", func(t *testing.T) {
		t.Parallel()
		fakeLocalResponse := "non-xml response"

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(nil)

		assert.EqualError(t, err, "unable to parse server response: EOF")
	})

	t.Run("server_broken_xml_response", func(t *testing.T) {
		t.Parallel()
		fakeLocalResponse := "<broken></xml><response>"

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(nil)

		assert.EqualError(t, err, "unable to parse server response: expected element type <ApiResponse> but have <broken>")
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
		t.Parallel()
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2050900">Invalid Address</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.getlist</RequestedCommand>
				<Server>PHX01SBAPIEXT05</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.011</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetList(nil)

		assert.EqualError(t, err, "Invalid Address (2050900)")
	})
}

func TestDomain_String(t *testing.T) {
	t.Parallel()
	t.Run("with_all_fields", func(t *testing.T) {
		t.Parallel()
		createdDate, _ := time.Parse("01/02/2006", "06/02/2021")
		expiresDate, _ := time.Parse("01/02/2006", "06/02/2022")
		d := Domain{
			ID:         String("677625"),
			Name:       String("domain.com"),
			User:       String("user"),
			Created:    &DateTime{createdDate},
			Expires:    &DateTime{expiresDate},
			IsExpired:  Bool(false),
			IsLocked:   Bool(false),
			AutoRenew:  Bool(true),
			WhoisGuard: String("ENABLED"),
			IsPremium:  Bool(false),
			IsOurDNS:   Bool(false),
		}
		result := d.String()
		assert.Contains(t, result, "677625")
		assert.Contains(t, result, "domain.com")
		assert.Contains(t, result, "user")
		assert.Contains(t, result, "ENABLED")
	})

	t.Run("nil_fields_do_not_panic", func(t *testing.T) {
		t.Parallel()
		d := Domain{}
		assert.NotPanics(t, func() { _ = d.String() })
	})
}
