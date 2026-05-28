package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsDNSService_SetEmailForwarding(t *testing.T) {
	t.Parallel()
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.setEmailForwarding</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.setEmailForwarding">
				<DomainEmailForwardingResult Domain="example.com" IsSuccess="true" />
			</CommandResponse>
			<Server>PHX01SBAPIEXT06</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.123</ExecutionTime>
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

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
		})
		if err != nil {
			t.Fatal("Unable to set email forwarding", err)
		}

		assert.Equal(t, "namecheap.domains.dns.setEmailForwarding", sentBody.Get("Command"))
	})

	t.Run("request_data_domain", func(t *testing.T) {
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

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
		})
		if err != nil {
			t.Fatal("Unable to set email forwarding", err)
		}

		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
	})

	t.Run("request_data_single_forward", func(t *testing.T) {
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

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
		})
		if err != nil {
			t.Fatal("Unable to set email forwarding", err)
		}

		assert.Equal(t, "info", sentBody.Get("mailbox1"))
		assert.Equal(t, "user@gmail.com", sentBody.Get("ForwardTo1"))
		assert.Empty(t, sentBody.Get("mailbox2"))
	})

	t.Run("request_data_multiple_forwards", func(t *testing.T) {
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

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
			{Mailbox: "support", ForwardTo: "support@company.com"},
			{Mailbox: "billing", ForwardTo: "billing@company.com"},
		})
		if err != nil {
			t.Fatal("Unable to set email forwarding", err)
		}

		assert.Equal(t, "info", sentBody.Get("mailbox1"))
		assert.Equal(t, "user@gmail.com", sentBody.Get("ForwardTo1"))
		assert.Equal(t, "support", sentBody.Get("mailbox2"))
		assert.Equal(t, "support@company.com", sentBody.Get("ForwardTo2"))
		assert.Equal(t, "billing", sentBody.Get("mailbox3"))
		assert.Equal(t, "billing@company.com", sentBody.Get("ForwardTo3"))
	})

	t.Run("correct_parsing_result", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
		})
		if err != nil {
			t.Fatal("Unable to set email forwarding", err)
		}

		assert.Equal(t, "example.com", *result.DomainDNSSetEmailForwardingResult.Domain)
		if assert.NotNil(t, result.DomainDNSSetEmailForwardingResult.IsSuccess) {
			assert.True(t, *result.DomainDNSSetEmailForwardingResult.IsSuccess)
		}
	})

	t.Run("request_data_empty_forwards", func(t *testing.T) {
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

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{})
		if err != nil {
			t.Fatal("Unable to set email forwarding", err)
		}

		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Empty(t, sentBody.Get("mailbox1"))
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
		t.Parallel()
		errorResponse := `
			<?xml version="1.0" encoding="utf-8"?>
			<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
				<Errors>
					<Error Number="2019166">Domain is not associated with your account</Error>
				</Errors>
				<Warnings />
				<RequestedCommand>namecheap.domains.dns.setEmailForwarding</RequestedCommand>
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

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
		})
		assert.EqualError(t, err, "Domain is not associated with your account (2019166)")
	})

	t.Run("doxml_failure_bad_url", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "://bad"

		_, err := client.DomainsDNS.SetEmailForwarding("example.com", []EmailForward{
			{Mailbox: "info", ForwardTo: "user@gmail.com"},
		})
		assert.Error(t, err)
	})
}
