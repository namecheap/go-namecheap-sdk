package namecheap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainsDNSSetCustom(t *testing.T) {
	t.Parallel()
	fakeResponse := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.setcustom</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.setCustom">
				<DomainDNSSetCustomResult Domain="domain.net" Updated="true" />
			</CommandResponse>
			<Server>PHX01SBAPIEXT06</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>2.599</ExecutionTime>
		</ApiResponse>`

	fakeNameservers := []string{"dns1.nameserver.com", "dns2.nameserver.com"}

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

		_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", fakeNameservers)
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.dns.setCustom", sentBody.Get("Command"))
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

		_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", fakeNameservers)
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "net", sentBody.Get("TLD"))
		assert.Equal(t, "domain", sentBody.Get("SLD"))
	})

	t.Run("request_data_nameservers", func(t *testing.T) {
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

		_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", fakeNameservers)
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		expectedNameservers := strings.Join(fakeNameservers, ",")

		assert.Equal(t, expectedNameservers, sentBody.Get("Nameservers"))
	})

	t.Run("correct_parsing_result_attributes", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		result, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", fakeNameservers)
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "domain.net", *result.DomainDNSSetCustomResult.Domain)
		assert.Equal(t, true, *result.DomainDNSSetCustomResult.Updated)
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
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

		_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", fakeNameservers)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "2019166")
	})

	t.Run("doxml_failure_bad_url", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "://bad"

		_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", fakeNameservers)
		assert.Error(t, err)
	})

	t.Run("invalid_domain_error", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)

		_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "invalid", fakeNameservers)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid domain")
	})

	errorCases := []struct {
		Nameservers []string
	}{
		{Nameservers: []string{}},
		{Nameservers: []string{"name.server"}},
	}

	for _, errorCase := range errorCases {
		t.Run("request_data_error_"+strconv.Itoa(len(errorCase.Nameservers))+"_nameservers", func(t *testing.T) {
			t.Parallel()
			mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(fakeResponse))
			}))
			defer mockServer.Close()

			client := setupClient(nil)
			client.BaseURL = mockServer.URL

			_, err := client.DomainsDNS.SetCustomWithContext(context.Background(), "domain.net", errorCase.Nameservers)

			assert.EqualError(t, err, "invalid nameservers: must contain minimum two items")
		})
	}
}

func TestDomainsDNSSetCustomResult_String(t *testing.T) {
	t.Parallel()
	t.Run("with_all_fields", func(t *testing.T) {
		t.Parallel()
		d := DomainsDNSSetCustomResult{
			Domain:  String("domain.net"),
			Updated: Bool(true),
		}
		result := d.String()
		assert.Contains(t, result, "domain.net")
		assert.Contains(t, result, "true")
	})

	t.Run("nil_fields_do_not_panic", func(t *testing.T) {
		t.Parallel()
		d := DomainsDNSSetCustomResult{}
		assert.NotPanics(t, func() { _ = d.String() })
	})
}
