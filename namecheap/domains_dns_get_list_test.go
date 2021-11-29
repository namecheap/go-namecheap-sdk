package namecheap

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDomainsDNSGetList(t *testing.T) {
	fakeResponse := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.getlist</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.getList">
				<DomainDNSGetListResult Domain="domain.net" IsUsingOurDNS="true" IsPremiumDNS="false" IsUsingFreeDNS="false">
					<Nameserver>dns1.registrar-servers.com</Nameserver>
					<Nameserver>dns2.registrar-servers.com</Nameserver>
				</DomainDNSGetListResult>
			</CommandResponse>
			<Server>PHX01SBAPIEXT05</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.565</ExecutionTime>
		</ApiResponse>`

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

		_, err := client.DomainsDNS.GetList("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.dns.getList", sentBody.Get("Command"))
	})

	t.Run("request_data_domain", func(t *testing.T) {
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

		_, err := client.DomainsDNS.GetList("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "net", sentBody.Get("TLD"))
		assert.Equal(t, "domain", sentBody.Get("SLD"))
	})

	t.Run("correct_parsing_result_attributes", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetList("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, false, *result.DomainDNSGetListResult.IsUsingFreeDNS)
		assert.Equal(t, false, *result.DomainDNSGetListResult.IsPremiumDNS)
		assert.Equal(t, true, *result.DomainDNSGetListResult.IsUsingOurDNS)
		assert.Equal(t, "domain.net", *result.DomainDNSGetListResult.Domain)
	})

	t.Run("correct_parsing_list", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetList("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		expectedNameservers := &[]string{"dns1.registrar-servers.com", "dns2.registrar-servers.com"}

		assert.Equal(t, expectedNameservers, result.DomainDNSGetListResult.Nameservers)
	})

	t.Run("empty_list", func(t *testing.T) {
		fakeLocalResponse := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.getlist</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.getList">
				<DomainDNSGetListResult Domain="ifree92.net" IsUsingOurDNS="true" IsPremiumDNS="false" IsUsingFreeDNS="false"/>
			</CommandResponse>
			<Server>PHX01SBAPIEXT05</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.565</ExecutionTime>
		</ApiResponse>`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeLocalResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetList("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Nil(t, result.DomainDNSGetListResult.Nameservers)
	})

	t.Run("FreeDNS domain handling", func(t *testing.T) {
		fakeDnsGetListResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2019166">Domain name not found</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.dns.getlist</RequestedCommand>
				<CommandResponse Type="namecheap.domains.dns.getList" />
				<Server>PHX01APIEXT11</Server>
				<GMTTimeDifference>--5:00</GMTTimeDifference>
				<ExecutionTime>0.074</ExecutionTime>
			</ApiResponse>
		`

		fakeGetInfoResponse := `
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
						</DnsDetails>
						<Modificationrights />
					</DomainGetInfoResult>
				</CommandResponse>
				<Server>PHX01APIEXT12</Server>
				<GMTTimeDifference>--5:00</GMTTimeDifference>
				<ExecutionTime>0.013</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			if query.Get("Command") == "namecheap.domains.dns.getList" {
				_, _ = writer.Write([]byte(fakeDnsGetListResponse))
			} else {
				_, _ = writer.Write([]byte(fakeGetInfoResponse))
			}
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetList("horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, true, *result.DomainDNSGetListResult.IsUsingFreeDNS)
		assert.Equal(t, false, *result.DomainDNSGetListResult.IsPremiumDNS)
		assert.Equal(t, true, *result.DomainDNSGetListResult.IsUsingOurDNS)
		assert.Equal(t, "horse-family.com.ua", *result.DomainDNSGetListResult.Domain)

		expectedNameservers := &[]string{"freedns1.registrar-servers.com", "freedns2.registrar-servers.com"}
		assert.Equal(t, expectedNameservers, result.DomainDNSGetListResult.Nameservers)
	})
}
