package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const domainsGetContactsOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.getContacts</RequestedCommand>
		<CommandResponse Type="namecheap.domains.getContacts">
			<DomainContactsResult Domain="example.com" domainnameid="30586" Readonly="false">
				<Registrant>
					<OrganizationName>NameCheap.com</OrganizationName>
					<JobTitle>CEO</JobTitle>
					<FirstName>John</FirstName>
					<LastName>Smith</LastName>
					<Address1>8939 S.cross Blvd</Address1>
					<Address2>Suite 600</Address2>
					<City>Phoenix</City>
					<StateProvince>AZ</StateProvince>
					<PostalCode>85284</PostalCode>
					<Country>US</Country>
					<Phone>+1.6613102107</Phone>
					<EmailAddress>john@example.com</EmailAddress>
				</Registrant>
				<Tech>
					<FirstName>Tech</FirstName>
					<LastName>Person</LastName>
					<EmailAddress>tech@example.com</EmailAddress>
				</Tech>
				<Admin>
					<FirstName>Admin</FirstName>
					<LastName>Person</LastName>
				</Admin>
				<AuxBilling>
					<FirstName>Billing</FirstName>
					<LastName>Person</LastName>
				</AuxBilling>
			</DomainContactsResult>
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>0.5</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_GetContacts(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsGetContactsOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetContactsWithContext(context.Background(), "example.com")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.getContacts", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))

		result := resp.DomainContactsResult
		assert.Equal(t, "example.com", *result.Domain)
		assert.Equal(t, 30586, *result.DomainNameID)
		assert.Equal(t, false, *result.ReadOnly)
		assert.Equal(t, "John", result.Registrant.FirstName)
		assert.Equal(t, "john@example.com", result.Registrant.EmailAddress)
		assert.Equal(t, "NameCheap.com", result.Registrant.OrganizationName)
		assert.Equal(t, "tech@example.com", result.Tech.EmailAddress)
		assert.Equal(t, "Admin", result.Admin.FirstName)
		assert.Equal(t, "Billing", result.AuxBilling.FirstName)
	})

	t.Run("empty_domain", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.GetContactsWithContext(context.Background(), "")
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
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

		_, err := client.Domains.GetContactsWithContext(context.Background(), "notfound.com")
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
		assert.True(t, errors.Is(err, ErrDomainNotFound))
	})
}
