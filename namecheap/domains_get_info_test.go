package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsGetInfo(t *testing.T) {
	t.Parallel()
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.getinfo</RequestedCommand>
			<CommandResponse Type="namecheap.domains.getInfo">
				<DomainGetInfoResult ID="1706717" DomainName="horse-family.com.ua" OwnerName="NCStaffvladlenf" IsOwner="false" IsPremium="false">
					<DomainDetails>
						<CreatedDate>11/26/2021</CreatedDate>
						<NumYears>0</NumYears>
					</DomainDetails>
					<LockDetails />
					<Whoisguard Enabled="NotAlloted">
						<ID>0</ID>
					</Whoisguard>
					<PremiumDnsSubscription>
						<UseAutoRenew>false</UseAutoRenew>
						<SubscriptionId>-1</SubscriptionId>
						<CreatedDate>0001-01-01T00:00:00</CreatedDate>
						<ExpirationDate>0001-01-01T00:00:00</ExpirationDate>
						<IsActive>false</IsActive>
					</PremiumDnsSubscription>
					<DnsDetails ProviderType="FreeDNS" IsUsingOurDNS="true" HostCount="0" EmailType="No Email Service" DynamicDNSStatus="false" IsFailover="false">
						<Nameserver>freedns1.registrar-servers.com</Nameserver>
						<Nameserver>freedns2.registrar-servers.com</Nameserver>
						<Nameserver>freedns3.registrar-servers.com</Nameserver>
						<Nameserver>freedns4.registrar-servers.com</Nameserver>
						<Nameserver>freedns5.registrar-servers.com</Nameserver>
					</DnsDetails>
					<Modificationrights />
				</DomainGetInfoResult>
			</CommandResponse>
			<Server>PHX01APIEXT12</Server>
			<GMTTimeDifference>--5:00</GMTTimeDifference>
			<ExecutionTime>0.013</ExecutionTime>
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

		_, err := client.Domains.GetInfo("horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.getInfo", sentBody.Get("Command"))
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

		_, err := client.DomainsDNS.GetHosts("horse-family.com.ua")

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

		_, err := client.DomainsDNS.GetHosts("domain.net")

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

		_, err := client.DomainsDNS.GetHosts("domain.net")

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

	t.Run("domains_get_info_error_response", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="2019166">Domain not found</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetInfo("notfound.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "2019166")
	})

	t.Run("domains_get_info_doxml_failure", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "://bad"

		_, err := client.Domains.GetInfo("domain.com")
		assert.Error(t, err)
	})
}
