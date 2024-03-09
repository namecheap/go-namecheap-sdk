package namecheap

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainNameserversCreate(t *testing.T) {
	fakeResponse := `
		<?xml version="1.0" encoding="UTF-8"?>
		<ApiResponse xmlns="http://api.namecheap.com/xml.response" Status="OK">
			<Errors />
			<RequestedCommand>namecheap.domains.ns.create</RequestedCommand>
			<CommandResponse Type="namecheap.domains.ns.create">
				<DomainNSCreateResult Domain="domain.com" Nameserver="ns1.domain.com" IP="1.1.1.1" IsSuccess="true" />
			</CommandResponse> 
			<Server>SERVER-NAME</Server>
			<GMTTimeDifference>+5</GMTTimeDifference>
			<ExecutionTime>32.76</ExecutionTime>
		</ApiResponse>
	`

	t.Run("request_command", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")
		if err != nil {
			t.Fatal("Unable to get domain nameserver", err)
		}

		assert.Equal(t, "namecheap.domains.ns.create", sentBody.Get("Command"))
	})

	t.Run("server_empty_response", func(t *testing.T) {
		fakeLocalResponse := ""

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "unable to parse server response: EOF")
	})

	t.Run("server_non_xml_response", func(t *testing.T) {
		fakeLocalResponse := "non-xml response"

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "unable to parse server response: EOF")
	})

	t.Run("server_broken_xml_response", func(t *testing.T) {
		fakeLocalResponse := "<broken></xml><response>"

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "unable to parse server response: expected element type <ApiResponse> but have <broken>")
	})

	t.Run("server_respond_with_domain_not_found_error", func(t *testing.T) {
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2019166">Domain not found</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.ns.create</RequestedCommand>
				<Server>PHX01SBAPIEXT05</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.011</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "Domain not found (2019166)")
	})

	t.Run("server_respond_with_domain_not_associated_with_account_error", func(t *testing.T) {
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2016166">Domain is not associated with your account</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.ns.create</RequestedCommand>
				<Server>PHX01SBAPIEXT05</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.011</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "Domain is not associated with your account (2016166)")
	})

	t.Run("server_respond_with_error_from_enom", func(t *testing.T) {
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="3031510">Error From Enom when Errorcount &lt;&gt; 0</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.ns.create</RequestedCommand>
				<Server>PHX01SBAPIEXT05</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.011</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "Error From Enom when Errorcount <> 0 (3031510)")
	})

	t.Run("server_respond_with_unknown_error_from_enom", func(t *testing.T) {
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="3050900">Unknown error from Enom</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.ns.create</RequestedCommand>
				<Server>PHX01SBAPIEXT05</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.011</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsNS.Create("specific-sld", "specific-tld", "ns1.domain.com", "1.1.1.1")

		assert.EqualError(t, err, "Unknown error from Enom (3050900)")
	})
}
