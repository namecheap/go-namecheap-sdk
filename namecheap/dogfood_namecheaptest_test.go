package namecheap_test

// This file dogfoods the exported namecheaptest package: a representative subset
// of the SDK's own tests, one per service (domains, domains.dns, ssl, users),
// migrated off hand-rolled httptest servers and inline XML onto namecheaptest.
// They assert exactly what the originals did, proving zero behavioral loss.
//
// The migration is deliberately scoped: it lives in the external test package
// (namecheap_test) because namecheaptest imports namecheap, so an internal test
// file importing it would create a cycle. Full migration of the remaining ~40
// test files is a documented follow-up (see CHANGELOG) to avoid a risky one-PR
// refactor and any coverage regression.

import (
	"context"
	"errors"
	"testing"
	"time"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
	"github.com/namecheap/go-namecheap-sdk/v2/namecheaptest"
	"github.com/stretchr/testify/assert"
)

// Domains service: migrated from TestDomainsService_GetRegistrarLock.
func TestDogfood_Domains_GetRegistrarLock(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubFixture("namecheap.domains.getRegistrarLock", "domains_getRegistrarLock")

		resp, err := srv.Client().Domains.GetRegistrarLockWithContext(context.Background(), "example.com")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "example.com", *resp.DomainGetRegistrarLockResult.Domain)
		assert.Equal(t, true, *resp.DomainGetRegistrarLockResult.RegistrarLockStatus)
		srv.AssertCalled(t, "namecheap.domains.getRegistrarLock", map[string]string{
			"Command":    "namecheap.domains.getRegistrarLock",
			"DomainName": "example.com",
		})
	})

	t.Run("empty_domain", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		_, err := srv.Client().Domains.GetRegistrarLockWithContext(context.Background(), "")
		var argErr *namecheap.InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubError("namecheap.domains.getRegistrarLock", 2019166, "Domain not found")

		_, err := srv.Client().Domains.GetRegistrarLockWithContext(context.Background(), "notfound.com")
		var apiErr *namecheap.APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2019166, apiErr.Number)
		}
	})
}

// domains.dns service: migrated from TestDomainsDNSSetDefault.
func TestDogfood_DomainsDNS_SetDefault(t *testing.T) {
	t.Parallel()

	t.Run("request_command_and_domain_split", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubFixture("namecheap.domains.dns.setDefault", "domains_dns_setDefault")

		_, err := srv.Client().DomainsDNS.SetDefaultWithContext(context.Background(), "domain.net")
		if err != nil {
			t.Fatal("Unable to set default DNS", err)
		}

		srv.AssertCalled(t, "namecheap.domains.dns.setDefault", map[string]string{
			"Command": "namecheap.domains.dns.setDefault",
			"TLD":     "net",
			"SLD":     "domain",
		})
	})

	t.Run("correct_parsing_result_attributes", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubFixture("namecheap.domains.dns.setDefault", "domains_dns_setDefault")

		result, err := srv.Client().DomainsDNS.SetDefaultWithContext(context.Background(), "domain.net")
		if err != nil {
			t.Fatal("Unable to set default DNS", err)
		}
		assert.Equal(t, "domain.net", *result.DomainDNSSetDefaultResult.Domain)
		assert.Equal(t, true, *result.DomainDNSSetDefaultResult.Updated)
	})

	t.Run("server_respond_with_error", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubError("namecheap.domains.dns.setDefault", 2019166, "Domain not found")

		_, err := srv.Client().DomainsDNS.SetDefaultWithContext(context.Background(), "notfound.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "2019166")
	})

	t.Run("doxml_failure_bad_url", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		client := srv.Client()
		client.BaseURL = "://bad"

		_, err := client.DomainsDNS.SetDefaultWithContext(context.Background(), "domain.net")
		assert.Error(t, err)
	})

	t.Run("invalid_domain_error", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		_, err := srv.Client().DomainsDNS.SetDefaultWithContext(context.Background(), "invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid domain")
	})
}

// ssl service: migrated from TestSSLService_GetInfo.
func TestDogfood_SSL_GetInfo(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_issued", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		// The shipped fixture expires in 2099, so IsExpiringSoon(30d) is false.
		srv.StubFixture("namecheap.ssl.getInfo", "ssl_getInfo")

		resp, err := srv.Client().SSL.GetInfoWithContext(context.Background(), 123456, "true", "Individual")
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		srv.AssertCalled(t, "namecheap.ssl.getInfo", map[string]string{
			"Command":           "namecheap.ssl.getInfo",
			"CertificateID":     "123456",
			"Returncertificate": "true",
			"Returntype":        "Individual",
		})

		result := resp.SSLGetInfoResult
		assert.Equal(t, 123456, *result.CertificateID)
		assert.Equal(t, "example.com", *result.CommonName)
		assert.Equal(t, namecheap.CertStatusActive, result.CertStatus())
		assert.True(t, result.IsIssued())
		assert.False(t, result.IsExpiringSoon(30*24*time.Hour))
	})

	t.Run("expiring_soon_detected", func(t *testing.T) {
		t.Parallel()
		soon := time.Now().Add(5 * 24 * time.Hour).Format("01/02/2006")
		srv := namecheaptest.NewServer(t)
		srv.Stub("namecheap.ssl.getInfo", `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.ssl.getInfo">
		<SSLGetInfoResult CertificateID="123456" Status="Active" Type="PositiveSSL" CommonName="example.com" Provider="Comodo" IssuedOn="09/22/2020" Expires="`+soon+`" />
	</CommandResponse>
</ApiResponse>`)

		resp, err := srv.Client().SSL.GetInfoWithContext(context.Background(), 123456, "", "")
		assert.NoError(t, err)
		assert.True(t, resp.SSLGetInfoResult.IsExpiringSoon(30*24*time.Hour))
	})

	t.Run("invalid_id_no_http", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		_, err := srv.Client().SSL.GetInfoWithContext(context.Background(), 0, "", "")
		var argErr *namecheap.InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "CertificateID")
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubError("namecheap.ssl.getInfo", 2033409, "Order not found")

		_, err := srv.Client().SSL.GetInfoWithContext(context.Background(), 999, "", "")
		var apiErr *namecheap.APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2033409, apiErr.Number)
		}
	})
}

// users service: migrated from TestUsersService_GetBalances(+_APIError).
func TestDogfood_Users_GetBalances(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubFixture("namecheap.users.getBalances", "users_getBalances")

		resp, err := srv.Client().Users.GetBalancesWithContext(context.Background())
		assert.NoError(t, err)
		srv.AssertCalled(t, "namecheap.users.getBalances", map[string]string{
			"Command": "namecheap.users.getBalances",
		})

		result := resp.UserGetBalancesResult
		if assert.NotNil(t, result) {
			assert.Equal(t, "USD", result.Currency)
			// 123.45 has no exact binary float representation; the Amount type
			// preserves the exact string (never parsed to float64).
			assert.Equal(t, namecheap.Amount("123.45"), result.AvailableBalance)
			assert.Equal(t, "123.45", result.AvailableBalance.String())
			assert.Equal(t, namecheap.Amount("123.45"), result.AccountBalance)
			assert.Equal(t, namecheap.Amount("15.00"), result.EarnedAmount)
			assert.Equal(t, namecheap.Amount("100.00"), result.WithdrawableAmount)
			assert.Equal(t, namecheap.Amount("42.50"), result.FundsRequiredForAutoRenew)
		}
	})

	t.Run("api_error", func(t *testing.T) {
		t.Parallel()
		srv := namecheaptest.NewServer(t)
		srv.StubError("namecheap.users.getBalances", 4011103, "API Key is invalid or API access has not been enabled")

		_, err := srv.Client().Users.GetBalancesWithContext(context.Background())
		var apiErr *namecheap.APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 4011103, apiErr.Number)
		}
	})
}
