package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

const domainsSetContactsOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.setContacts</RequestedCommand>
		<CommandResponse Type="namecheap.domains.setContacts">
			<DomainSetContactResult Domain="example.com" IsSuccess="true" />
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>0.5</ExecutionTime>
	</ApiResponse>
`

func validSetContactsArgs() *DomainsSetContactsArgs {
	return &DomainsSetContactsArgs{
		DomainName: "example.com",
		Registrant: validContactInfo(),
		Tech:       validContactInfo(),
		Admin:      validContactInfo(),
		AuxBilling: validContactInfo(),
	}
}

func TestDomainsService_SetContacts(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsSetContactsOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.SetContactsWithContext(context.Background(), validSetContactsArgs())
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.setContacts", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "John", sentBody.Get("RegistrantFirstName"))
		assert.Equal(t, "Suite 600", sentBody.Get("TechAddress2"))
		assert.Equal(t, "example.com", *resp.DomainSetContactResult.Domain)
		assert.Equal(t, true, *resp.DomainSetContactResult.IsSuccess)
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.SetContactsWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("missing_contact_fields_reported_all_at_once", func(t *testing.T) {
		t.Parallel()
		var called int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&called, 1)
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		args := validSetContactsArgs()
		args.Registrant.FirstName = ""
		args.Registrant.LastName = ""
		args.AuxBilling.Country = ""

		_, err := client.Domains.SetContactsWithContext(context.Background(), args)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "RegistrantFirstName")
			assert.Contains(t, argErr.Fields, "RegistrantLastName")
			assert.Contains(t, argErr.Fields, "AuxBillingCountry")
			assert.GreaterOrEqual(t, len(argErr.Fields), 3, "every missing field must be listed, not just the first")
		}
		assert.Equal(t, int32(0), atomic.LoadInt32(&called), "no HTTP call may happen when validation fails")
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="2016166">Domain is not associated with your account</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.SetContactsWithContext(context.Background(), validSetContactsArgs())
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2016166, apiErr.Number)
		}
	})
}
