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
	"time"

	"github.com/stretchr/testify/assert"
)

// validContactInfo returns a fully-populated ContactInfo for use as any of the
// four contact blocks in create/setContacts tests.
func validContactInfo() ContactInfo {
	return ContactInfo{
		FirstName:        "John",
		LastName:         "Smith",
		Address1:         "8939 S.cross Blvd",
		City:             "Phoenix",
		StateProvince:    "AZ",
		PostalCode:       "85284",
		Country:          "US",
		Phone:            "+1.6613102107",
		EmailAddress:     "john@example.com",
		OrganizationName: "NameCheap.com",
		JobTitle:         "Dev",
		Address2:         "Suite 600",
	}
}

// validCreateArgs returns create args that pass validation, ready to be tweaked
// per test.
func validCreateArgs() *DomainsCreateArgs {
	return &DomainsCreateArgs{
		DomainName: "example.com",
		Years:      2,
		Registrant: validContactInfo(),
		Tech:       validContactInfo(),
		Admin:      validContactInfo(),
		AuxBilling: validContactInfo(),
	}
}

const domainsCreateOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.create</RequestedCommand>
		<CommandResponse Type="namecheap.domains.create">
			<DomainCreateResult Domain="example.com" Registered="true" ChargedAmount="10.87" DomainID="1234567" OrderID="987654" TransactionID="112233" WhoisguardEnable="true" NonRealTimeDomain="false" />
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>1.234</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(domainsCreateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.CreateWithContext(context.Background(), validCreateArgs())
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.create", sentBody.Get("Command"))
		assert.Equal(t, "example.com", sentBody.Get("DomainName"))
		assert.Equal(t, "2", sentBody.Get("Years"))
		assert.Equal(t, "John", sentBody.Get("RegistrantFirstName"))
		assert.Equal(t, "john@example.com", sentBody.Get("AuxBillingEmailAddress"))

		result := resp.DomainCreateResult
		assert.Equal(t, "example.com", *result.Domain)
		assert.Equal(t, true, *result.Registered)
		assert.Equal(t, 1234567, *result.DomainID)
		assert.Equal(t, 987654, *result.OrderID)
		assert.Equal(t, 112233, *result.TransactionID)
	})

	t.Run("money_preserved_exactly_as_string", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(domainsCreateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.CreateWithContext(context.Background(), validCreateArgs())
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		// 10.87 cannot be represented exactly in binary floating point; assert the
		// exact string is preserved by the Amount type (never parsed to float64).
		assert.Equal(t, Amount("10.87"), *resp.DomainCreateResult.ChargedAmount)
		assert.Equal(t, "10.87", resp.DomainCreateResult.ChargedAmount.String())
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Domains.CreateWithContext(context.Background(), nil)
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

		args := validCreateArgs()
		args.Registrant.FirstName = ""
		args.Tech.EmailAddress = ""
		args.Admin.City = ""

		_, err := client.Domains.CreateWithContext(context.Background(), args)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "RegistrantFirstName")
			assert.Contains(t, argErr.Fields, "TechEmailAddress")
			assert.Contains(t, argErr.Fields, "AdminCity")
			assert.Contains(t, err.Error(), "RegistrantFirstName")
			assert.Contains(t, err.Error(), "TechEmailAddress")
			assert.Contains(t, err.Error(), "AdminCity")
		}
		assert.Equal(t, int32(0), atomic.LoadInt32(&called), "no HTTP call may happen when validation fails")
	})

	t.Run("premium_guard_blocks_missing_price_no_http", func(t *testing.T) {
		t.Parallel()
		var called int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&called, 1)
			t.Errorf("server must not be called when the premium guard trips")
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		args := validCreateArgs()
		args.IsPremiumDomain = true // PremiumPrice deliberately left empty

		_, err := client.Domains.CreateWithContext(context.Background(), args)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "PremiumPrice")
		}
		assert.Equal(t, int32(0), atomic.LoadInt32(&called), "no charge-bearing call may happen")
	})

	t.Run("premium_guard_blocks_inconsistent_pricing", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0" // must never be reached

		args := validCreateArgs()
		args.IsPremiumDomain = false
		args.PremiumPrice = Amount("99.99")

		_, err := client.Domains.CreateWithContext(context.Background(), args)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "PremiumPrice")
		}
	})

	t.Run("premium_success_sends_price", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsCreateOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		args := validCreateArgs()
		args.IsPremiumDomain = true
		args.PremiumPrice = Amount("13000.0000")
		args.EapFee = Amount("2.5000")

		_, err := client.Domains.CreateWithContext(context.Background(), args)
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		assert.Equal(t, "true", sentBody.Get("IsPremiumDomain"))
		assert.Equal(t, "13000.0000", sentBody.Get("PremiumPrice"))
		assert.Equal(t, "2.5000", sentBody.Get("EapFee"))
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="2011170">Promotion code is invalid</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.Domains.CreateWithContext(context.Background(), validCreateArgs())
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011170, apiErr.Number)
		}
		assert.True(t, errors.Is(err, ErrPromotionCodeInvalid))
	})

	t.Run("non_idempotent_no_retry_on_server_error", func(t *testing.T) {
		t.Parallel()
		var calls int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			_, _ = writer.Write([]byte(apiErrorXML(5050900, "Server exception")))
		}))
		defer mockServer.Close()

		client := newResilienceClient(mockServer.URL, func(o *ClientOptions) {
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.Domains.CreateWithContext(context.Background(), validCreateArgs())
		var apiErr *APIError
		assert.True(t, errors.As(err, &apiErr))
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a charge-bearing call must not be retried on an ambiguous server error")
		assert.NotContains(t, err.Error(), "after")
	})
}
