package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsDNSGetHosts(t *testing.T) {
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.gethosts</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.getHosts">
				<DomainDNSGetHostsResult Domain="domain.net" EmailType="MX" IsUsingOurDNS="true">
					<host HostId="877748" Name="host33" Type="MX" Address="addr.domain.com." MXPref="10" TTL="1800" AssociatedAppTitle="" FriendlyName="" IsActive="true" IsDDNSEnabled="false" />
					<host HostId="877749" Name="@" Type="CNAME" Address="anotherdomain.com" MXPref="10" TTL="1800" AssociatedAppTitle="" FriendlyName="" IsActive="true" IsDDNSEnabled="false" />
				</DomainDNSGetHostsResult>
			</CommandResponse>
			<Server>PHX01SBAPIEXT06</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.417</ExecutionTime>
		</ApiResponse>
	`

	t.Run("request_command", func(t *testing.T) {
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

		_, err := client.DomainsDNS.GetHosts("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.dns.getHosts", sentBody.Get("Command"))
	})

	t.Run("request_data_sld_tld", func(t *testing.T) {
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

		_, err := client.DomainsDNS.GetHosts("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "domain", sentBody.Get("SLD"))
		assert.Equal(t, "net", sentBody.Get("TLD"))
	})

	t.Run("request_data_error_incorrect_domain", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.GetHosts("domain")

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid domain: incorrect format")
	})

	t.Run("correct_parsing_result_tags", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		response, err := client.DomainsDNS.GetHosts("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "domain.net", *response.DomainDNSGetHostsResult.Domain)
		assert.Equal(t, "MX", *response.DomainDNSGetHostsResult.EmailType)
		assert.Equal(t, true, *response.DomainDNSGetHostsResult.IsUsingOurDNS)
	})

	t.Run("correct_parsing_list", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		response, err := client.DomainsDNS.GetHosts("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		expectedList := []DomainsDNSHostRecordDetailed{
			{
				HostId:             Int(877748),
				Name:               String("host33"),
				Type:               String("MX"),
				Address:            String("addr.domain.com."),
				MXPref:             Int(10),
				TTL:                Int(1800),
				AssociatedAppTitle: String(""),
				FriendlyName:       String(""),
				IsActive:           Bool(true),
				IsDDNSEnabled:      Bool(false),
			},
			{
				HostId:             Int(877749),
				Name:               String("@"),
				Type:               String("CNAME"),
				Address:            String("anotherdomain.com"),
				MXPref:             Int(10),
				TTL:                Int(1800),
				AssociatedAppTitle: String(""),
				FriendlyName:       String(""),
				IsActive:           Bool(true),
				IsDDNSEnabled:      Bool(false),
			},
		}

		assert.Equal(t, &expectedList, response.DomainDNSGetHostsResult.Hosts)
	})

	t.Run("empty_record_list", func(t *testing.T) {
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<Warnings />
				<RequestedCommand>namecheap.domains.dns.gethosts</RequestedCommand>
				<CommandResponse Type="namecheap.domains.dns.getHosts">
					<DomainDNSGetHostsResult Domain="domain.net" EmailType="MX" IsUsingOurDNS="true"/>
				</CommandResponse>
				<Server>PHX01SBAPIEXT06</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.417</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		response, err := client.DomainsDNS.GetHosts("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Nil(t, response.DomainDNSGetHostsResult.Hosts)
	})

	t.Run("server_empty_response", func(t *testing.T) {
		fakeLocalResponse := ""

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.GetHosts("domain.net")

		assert.EqualError(t, err, "unable to parse server response: EOF")
	})

	t.Run("server_non_xml_response", func(t *testing.T) {
		fakeLocalResponse := "non-xml response"

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.GetHosts("domain.net")

		assert.EqualError(t, err, "unable to parse server response: EOF")
	})

	t.Run("server_broken_xml_response", func(t *testing.T) {
		fakeLocalResponse := "<broken></xml><response>"

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.GetHosts("domain.net")

		assert.EqualError(t, err, "unable to parse server response: expected element type <ApiResponse> but have <broken>")
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
		fakeLocalResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2050900">Invalid Address</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.dns.getlist</RequestedCommand>
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

		_, err := client.DomainsDNS.GetHosts("domain.net")

		assert.EqualError(t, err, "Invalid Address (2050900)")
	})
}

func TestDomainsDNSHostRecordDetailed_String(t *testing.T) {
	t.Run("with_all_fields", func(t *testing.T) {
		d := DomainsDNSHostRecordDetailed{
			HostId:             Int(877748),
			Name:               String("host33"),
			Type:               String("MX"),
			Address:            String("addr.domain.com."),
			MXPref:             Int(10),
			TTL:                Int(1800),
			AssociatedAppTitle: String(""),
			FriendlyName:       String(""),
			IsActive:           Bool(true),
			IsDDNSEnabled:      Bool(false),
		}
		result := d.String()
		assert.Contains(t, result, "877748")
		assert.Contains(t, result, "host33")
		assert.Contains(t, result, "MX")
		assert.Contains(t, result, "addr.domain.com.")
	})

	t.Run("nil_fields_do_not_panic", func(t *testing.T) {
		d := DomainsDNSHostRecordDetailed{}
		assert.NotPanics(t, func() { _ = d.String() })
	})
}
