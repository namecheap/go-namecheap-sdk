package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsService_Check(t *testing.T) {
	t.Parallel()
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.check</RequestedCommand>
			<CommandResponse Type="namecheap.domains.check">
				<DomainCheckResult Domain="example.com" Available="true" ErrorNo="0" Description="" IsPremiumName="false" PremiumRegistrationPrice="0" PremiumRenewalPrice="0" PremiumRestorePrice="0" PremiumTransferPrice="0" IcannFee="0" EapFee="0" />
			</CommandResponse>
			<Server>PHX01SBAPIEXT06</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.417</ExecutionTime>
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

		_, err := client.Domains.Check("example.com")
		if err != nil {
			t.Fatal("Unable to check domain", err)
		}

		assert.Equal(t, "namecheap.domains.check", sentBody.Get("Command"))
	})

	t.Run("request_data_single_domain", func(t *testing.T) {
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

		_, err := client.Domains.Check("example.com")
		if err != nil {
			t.Fatal("Unable to check domain", err)
		}

		assert.Equal(t, "example.com", sentBody.Get("DomainList"))
	})

	t.Run("request_data_multiple_domains", func(t *testing.T) {
		t.Parallel()
		multiResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<Warnings />
				<RequestedCommand>namecheap.domains.check</RequestedCommand>
				<CommandResponse Type="namecheap.domains.check">
					<DomainCheckResult Domain="example.com" Available="true" ErrorNo="0" Description="" IsPremiumName="false" PremiumRegistrationPrice="0" PremiumRenewalPrice="0" PremiumRestorePrice="0" PremiumTransferPrice="0" IcannFee="0" EapFee="0" />
					<DomainCheckResult Domain="example.net" Available="false" ErrorNo="0" Description="" IsPremiumName="false" PremiumRegistrationPrice="0" PremiumRenewalPrice="0" PremiumRestorePrice="0" PremiumTransferPrice="0" IcannFee="0" EapFee="0" />
				</CommandResponse>
				<Server>PHX01SBAPIEXT06</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.417</ExecutionTime>
			</ApiResponse>
		`

		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(multiResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.Domains.Check("example.com", "example.net")
		if err != nil {
			t.Fatal("Unable to check domains", err)
		}

		assert.Equal(t, "example.com,example.net", sentBody.Get("DomainList"))
		assert.Len(t, *result.DomainCheckResults, 2)
		assert.Equal(t, "example.com", *(*result.DomainCheckResults)[0].Domain)
		assert.Equal(t, "example.net", *(*result.DomainCheckResults)[1].Domain)
	})

	t.Run("correct_parsing_result_attributes", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.Domains.Check("example.com")
		if err != nil {
			t.Fatal("Unable to check domain", err)
		}

		first := (*result.DomainCheckResults)[0]
		assert.Equal(t, "example.com", *first.Domain)
		assert.Equal(t, true, *first.IsAvailable)
		assert.Equal(t, false, *first.IsPremiumName)
		assert.Equal(t, 0.0, *first.PremiumRegistrationPrice)
		assert.Equal(t, 0.0, *first.PremiumRenewalPrice)
		assert.Equal(t, 0.0, *first.PremiumRestorePrice)
		assert.Equal(t, 0.0, *first.PremiumTransferPrice)
		assert.Equal(t, 0.0, *first.IcannFee)
		assert.Equal(t, 0.0, *first.EapFee)
	})

	t.Run("correct_parsing_premium_result_attributes", func(t *testing.T) {
		t.Parallel()
		premiumFakeResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<Warnings />
				<RequestedCommand>namecheap.domains.check</RequestedCommand>
				<CommandResponse Type="namecheap.domains.check">
					<DomainCheckResult Domain="example.com" Available="true" ErrorNo="0" Description="" IsPremiumName="true" PremiumRegistrationPrice="13000.0000" PremiumRenewalPrice="13000.0000" PremiumRestorePrice="6500.0000" PremiumTransferPrice="13000.0000" IcannFee="0.0000" EapFee="0.0000" />
				</CommandResponse>
				<Server>PHX01SBAPIEXT06</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.417</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(premiumFakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.Domains.Check("example.com")
		if err != nil {
			t.Fatal("Unable to check domain", err)
		}

		first := (*result.DomainCheckResults)[0]
		assert.Equal(t, "example.com", *first.Domain)
		assert.Equal(t, true, *first.IsAvailable)
		assert.Equal(t, true, *first.IsPremiumName)
		assert.Equal(t, 13000.0, *first.PremiumRegistrationPrice)
		assert.Equal(t, 13000.0, *first.PremiumRenewalPrice)
		assert.Equal(t, 6500.0, *first.PremiumRestorePrice)
		assert.Equal(t, 13000.0, *first.PremiumTransferPrice)
		assert.Equal(t, 0.0, *first.IcannFee)
		assert.Equal(t, 0.0, *first.EapFee)
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="1011150">Domain is invalid</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.Check("bad")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "1011150")
	})

	t.Run("doxml_failure_bad_url", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "://bad"

		_, err := client.Domains.Check("example.com")
		assert.Error(t, err)
	})
}
