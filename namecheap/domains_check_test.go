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

	t.Run("request_data_domain", func(t *testing.T) {
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

	t.Run("correct_parsing_result_attributes", func(t *testing.T) {
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

		assert.Equal(t, "example.com", *result.DomainCheckResult.Domain)
		assert.Equal(t, true, *result.DomainCheckResult.IsAvailable)
		assert.Equal(t, false, *result.DomainCheckResult.IsPremiumName)
		assert.Equal(t, 0.0, *result.DomainCheckResult.PremiumRegistrationPrice)
		assert.Equal(t, 0.0, *result.DomainCheckResult.PremiumRenewalPrice)
		assert.Equal(t, 0.0, *result.DomainCheckResult.PremiumRestorePrice)
		assert.Equal(t, 0.0, *result.DomainCheckResult.PremiumTransferPrice)
		assert.Equal(t, 0.0, *result.DomainCheckResult.IcannFee)
		assert.Equal(t, 0.0, *result.DomainCheckResult.EapFee)
	})

	t.Run("correct_parsing_premium_result_attributes", func(t *testing.T) {
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

		assert.Equal(t, "example.com", *result.DomainCheckResult.Domain)
		assert.Equal(t, true, *result.DomainCheckResult.IsAvailable)
		assert.Equal(t, true, *result.DomainCheckResult.IsPremiumName)
		assert.Equal(t, 13000.0, *result.DomainCheckResult.PremiumRegistrationPrice)
		assert.Equal(t, 13000.0, *result.DomainCheckResult.PremiumRenewalPrice)
		assert.Equal(t, 6500.0, *result.DomainCheckResult.PremiumRestorePrice)
		assert.Equal(t, 13000.0, *result.DomainCheckResult.PremiumTransferPrice)
		assert.Equal(t, 0.0, *result.DomainCheckResult.IcannFee)
		assert.Equal(t, 0.0, *result.DomainCheckResult.EapFee)
	})
}
