package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsDNSService_GetEmailForwarding(t *testing.T) {
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.getEmailForwarding</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.getEmailForwarding">
				<DomainEmailForwardingResult Domain="example.com">
					<Forward mailbox="info" ForwardTo="user@gmail.com" />
					<Forward mailbox="support" ForwardTo="support@company.com" />
				</DomainEmailForwardingResult>
			</CommandResponse>
			<Server>PHX01SBAPIEXT06</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.123</ExecutionTime>
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

		_, err := client.DomainsDNS.GetEmailForwarding("example.com")
		if err != nil {
			t.Fatal("Unable to get email forwarding", err)
		}

		assert.Equal(t, "namecheap.domains.dns.getEmailForwarding", sentBody.Get("Command"))
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

		_, err := client.DomainsDNS.GetEmailForwarding("example.com")
		if err != nil {
			t.Fatal("Unable to get email forwarding", err)
		}

		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
	})

	t.Run("correct_parsing_forwarding_rules", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetEmailForwarding("example.com")
		if err != nil {
			t.Fatal("Unable to get email forwarding", err)
		}

		assert.Equal(t, "example.com", *result.DomainEmailForwardingResult.Domain)
		assert.Len(t, *result.DomainEmailForwardingResult.Forwards, 2)

		forwards := *result.DomainEmailForwardingResult.Forwards
		assert.Equal(t, "info", forwards[0].Mailbox)
		assert.Equal(t, "user@gmail.com", forwards[0].ForwardTo)
		assert.Equal(t, "support", forwards[1].Mailbox)
		assert.Equal(t, "support@company.com", forwards[1].ForwardTo)
	})

	t.Run("empty_forwarding_list", func(t *testing.T) {
		emptyResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
				<Errors />
				<Warnings />
				<RequestedCommand>namecheap.domains.dns.getEmailForwarding</RequestedCommand>
				<CommandResponse Type="namecheap.domains.dns.getEmailForwarding">
					<DomainEmailForwardingResult Domain="example.com" />
				</CommandResponse>
				<Server>PHX01SBAPIEXT06</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.123</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(emptyResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.GetEmailForwarding("example.com")
		if err != nil {
			t.Fatal("Unable to get email forwarding", err)
		}

		assert.Equal(t, "example.com", *result.DomainEmailForwardingResult.Domain)
		assert.Nil(t, result.DomainEmailForwardingResult.Forwards)
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
		errorResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2019166">Domain is not associated with your account</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.dns.getEmailForwarding</RequestedCommand>
				<CommandResponse />
				<Server>PHX01SBAPIEXT06</Server>
				<GMTTimeDifference>--4:00</GMTTimeDifference>
				<ExecutionTime>0.123</ExecutionTime>
			</ApiResponse>
		`

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(errorResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.GetEmailForwarding("example.com")
		assert.EqualError(t, err, "Domain is not associated with your account (2019166)")
	})
}
