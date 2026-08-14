package namecheap

import (
	"context"
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

	fakePopulatedResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.getinfo</RequestedCommand>
			<CommandResponse Type="namecheap.domains.getInfo">
				<DomainGetInfoResult Status="Ok" ID="313319" DomainName="domain1.com" OwnerName="apisample" IsOwner="true" IsPremium="true">
					<DomainDetails>
						<CreatedDate>04/23/2016</CreatedDate>
						<ExpiredDate>04/23/2017</ExpiredDate>
						<NumYears>1</NumYears>
					</DomainDetails>
					<LockDetails />
					<Whoisguard Enabled="True">
						<ID>53536</ID>
						<ExpiredDate>04/23/2017</ExpiredDate>
						<EmailDetails WhoisGuardEmail="protect@whoisguard.com" ForwardedTo="example@gmail.com" LastAutoEmailChangeDate="05/20/2020" AutoEmailChangeFrequencyDays="3" />
					</Whoisguard>
					<PremiumDnsSubscription>
						<UseAutoRenew>true</UseAutoRenew>
						<SubscriptionId>4756</SubscriptionId>
						<CreatedDate>2016-04-23T00:00:00</CreatedDate>
						<ExpirationDate>2017-04-23T00:00:00</ExpirationDate>
						<IsActive>true</IsActive>
					</PremiumDnsSubscription>
					<DnsDetails ProviderType="CUSTOM" IsUsingOurDNS="false" HostCount="2" EmailType="FWD" DynamicDNSStatus="false" IsFailover="false">
						<Nameserver>dns1.name-servers.com</Nameserver>
						<Nameserver>dns2.name-servers.com</Nameserver>
					</DnsDetails>
					<Modificationrights All="false">
						<Rights Type="dns">OK</Rights>
						<Rights Type="eforward">OK</Rights>
						<Rights Type="rlock">OK</Rights>
						<Rights Type="parkpage">OK</Rights>
						<Rights Type="hosts">OK</Rights>
						<Rights Type="ddns">OK</Rights>
						<Rights Type="extend">OK</Rights>
						<Rights Type="nameserver">OK</Rights>
						<Rights Type="autorenew">OK</Rights>
						<Rights Type="whoisguard">OK</Rights>
						<Rights Type="premiumDns">OK</Rights>
						<Rights Type="dnsSec">OK</Rights>
					</Modificationrights>
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

		_, err := client.Domains.GetInfoWithContext(context.Background(), "horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.getInfo", sentBody.Get("Command"))
	})

	t.Run("parses_all_result_fields", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetInfoWithContext(context.Background(), "horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domain info", err)
		}

		result := resp.DomainDNSGetListResult
		if result == nil {
			t.Fatal("GetInfoWithContext() result is nil, want populated DomainsGetInfoResult")
		}

		assert.Equal(t, 1706717, *result.ID)
		assert.Equal(t, "horse-family.com.ua", *result.DomainName)
		assert.Equal(t, "NCStaffvladlenf", *result.OwnerName)
		assert.Equal(t, false, *result.IsOwner)
		assert.Equal(t, false, *result.IsPremium)
		assert.Nil(t, result.Status)

		if assert.NotNil(t, result.DomainDetails) {
			assert.Equal(t, "11/26/2021", result.DomainDetails.CreatedDate.Format("01/02/2006"))
			assert.Nil(t, result.DomainDetails.ExpiredDate)
			assert.Equal(t, 0, *result.DomainDetails.NumYears)
		}

		if assert.NotNil(t, result.WhoisGuard) {
			assert.Equal(t, "NotAlloted", *result.WhoisGuard.Enabled)
			assert.Equal(t, 0, *result.WhoisGuard.ID)
			assert.Nil(t, result.WhoisGuard.ExpiredDate)
			assert.Nil(t, result.WhoisGuard.EmailDetails)
		}

		if assert.NotNil(t, result.PremiumDnsSubscription) {
			assert.Equal(t, false, *result.PremiumDnsSubscription.UseAutoRenew)
			assert.Equal(t, -1, *result.PremiumDnsSubscription.SubscriptionID)
			assert.Equal(t, "0001-01-01T00:00:00", *result.PremiumDnsSubscription.CreatedDate)
			assert.Equal(t, "0001-01-01T00:00:00", *result.PremiumDnsSubscription.ExpirationDate)
			assert.Equal(t, false, *result.PremiumDnsSubscription.IsActive)
		}

		if assert.NotNil(t, result.DnsDetails) {
			assert.Equal(t, "FreeDNS", *result.DnsDetails.ProviderType)
			assert.Equal(t, true, *result.DnsDetails.IsUsingOurDNS)
			assert.Equal(t, 0, *result.DnsDetails.HostCount)
			assert.Equal(t, "No Email Service", *result.DnsDetails.EmailType)
			assert.Equal(t, false, *result.DnsDetails.DynamicDNSStatus)
			assert.Equal(t, false, *result.DnsDetails.IsFailover)
		}

		if assert.NotNil(t, result.ModificationRights) {
			assert.Nil(t, result.ModificationRights.All)
			assert.Nil(t, result.ModificationRights.Rights)
		}
	})

	t.Run("parses_populated_privacy_response", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakePopulatedResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetInfoWithContext(context.Background(), "domain1.com")
		if err != nil {
			t.Fatal("Unable to get domain info", err)
		}

		result := resp.DomainDNSGetListResult
		if result == nil {
			t.Fatal("GetInfoWithContext() result is nil, want populated DomainsGetInfoResult")
		}

		assert.Equal(t, "Ok", *result.Status)
		assert.Equal(t, 313319, *result.ID)
		assert.Equal(t, true, *result.IsOwner)
		assert.Equal(t, true, *result.IsPremium)

		if assert.NotNil(t, result.DomainDetails) {
			assert.Equal(t, "04/23/2017", result.DomainDetails.ExpiredDate.Format("01/02/2006"))
			assert.Equal(t, 1, *result.DomainDetails.NumYears)
		}

		if assert.NotNil(t, result.WhoisGuard) {
			assert.Equal(t, "True", *result.WhoisGuard.Enabled)
			assert.Equal(t, 53536, *result.WhoisGuard.ID)
			assert.Equal(t, "04/23/2017", result.WhoisGuard.ExpiredDate.Format("01/02/2006"))
			if assert.NotNil(t, result.WhoisGuard.EmailDetails) {
				assert.Equal(t, "protect@whoisguard.com", *result.WhoisGuard.EmailDetails.WhoisGuardEmail)
				assert.Equal(t, "example@gmail.com", *result.WhoisGuard.EmailDetails.ForwardedTo)
				assert.Equal(t, "05/20/2020", *result.WhoisGuard.EmailDetails.LastAutoEmailChangeDate)
				assert.Equal(t, 3, *result.WhoisGuard.EmailDetails.AutoEmailChangeFrequencyDays)
			}
		}

		if assert.NotNil(t, result.PremiumDnsSubscription) {
			assert.Equal(t, true, *result.PremiumDnsSubscription.UseAutoRenew)
			assert.Equal(t, 4756, *result.PremiumDnsSubscription.SubscriptionID)
		}

		if assert.NotNil(t, result.ModificationRights) {
			assert.Equal(t, false, *result.ModificationRights.All)
			if assert.NotNil(t, result.ModificationRights.Rights) {
				rights := *result.ModificationRights.Rights
				assert.Len(t, rights, 12)
				assert.Equal(t, "dns", *rights[0].Type)
				assert.Equal(t, "OK", *rights[0].Value)
				assert.Equal(t, "rlock", *rights[2].Type)
			}
		}
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

		_, err := client.DomainsDNS.GetHostsWithContext(context.Background(), "horse-family.com.ua")

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

		_, err := client.DomainsDNS.GetHostsWithContext(context.Background(), "domain.net")

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

		_, err := client.DomainsDNS.GetHostsWithContext(context.Background(), "domain.net")

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

		_, err := client.DomainsDNS.GetHostsWithContext(context.Background(), "domain.net")

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

		_, err := client.Domains.GetInfoWithContext(context.Background(), "notfound.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "2019166")
	})

	t.Run("domains_get_info_doxml_failure", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "://bad"

		_, err := client.Domains.GetInfoWithContext(context.Background(), "domain.com")
		assert.Error(t, err)
	})

	t.Run("result_accessor_returns_deprecated_field", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetInfoWithContext(context.Background(), "horse-family.com.ua")
		if err != nil {
			t.Fatal("Unable to get domain info", err)
		}

		assert.NotNil(t, resp.Result())
		assert.Same(t, resp.DomainDNSGetListResult, resp.Result())
	})
}
