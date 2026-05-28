package namecheap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsDNSSetDefault(t *testing.T) {
	fakeResponse := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.setdefault</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.setDefault">
				<DomainDNSSetDefaultResult Domain="domain.net" Updated="true" />
			</CommandResponse>
			<Server>PHX01SBAPIEXT05</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>2.975</ExecutionTime>
		</ApiResponse>`

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

		_, err := client.DomainsDNS.SetDefault("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.dns.setDefault", sentBody.Get("Command"))
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

		_, err := client.DomainsDNS.SetDefault("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "net", sentBody.Get("TLD"))
		assert.Equal(t, "domain", sentBody.Get("SLD"))
	})

	t.Run("correct_parsing_result_attributes", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.SetDefault("domain.net")
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "domain.net", *result.DomainDNSSetDefaultResult.Domain)
		assert.Equal(t, true, *result.DomainDNSSetDefaultResult.Updated)
	})
}

func TestDomainDNSSetDefaultResult_String(t *testing.T) {
	t.Run("with_all_fields", func(t *testing.T) {
		d := DomainDNSSetDefaultResult{
			Domain:  String("domain.net"),
			Updated: Bool(true),
		}
		result := d.String()
		assert.Contains(t, result, "domain.net")
		assert.Contains(t, result, "true")
	})

	t.Run("nil_fields_do_not_panic", func(t *testing.T) {
		d := DomainDNSSetDefaultResult{}
		assert.NotPanics(t, func() { _ = d.String() })
	})
}
