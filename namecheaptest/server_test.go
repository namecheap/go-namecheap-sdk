package namecheaptest_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
	"github.com/namecheap/go-namecheap-sdk/v2/namecheaptest"
	"github.com/stretchr/testify/assert"
)

func TestServer_StubFixtureAndAssertCalled(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.domains.getInfo", "domains_getInfo")

	client := srv.Client()
	resp, err := client.Domains.GetInfoWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
	if assert.NotNil(t, resp) && assert.NotNil(t, resp.DomainDNSGetListResult) {
		assert.NotNil(t, resp.DomainDNSGetListResult.DomainName)
	}

	// AssertCalled matches on a subset of the sent params.
	srv.AssertCalled(t, "namecheap.domains.getInfo", map[string]string{
		"DomainName": "example.com",
	})
}

func TestServer_StubWithFixtureOK(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	// Mirrors the issue's example API shape.
	srv.Stub("namecheap.domains.getInfo", namecheaptest.FixtureOK("domains_getInfo"))

	_, err := srv.Client().Domains.GetInfoWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
}

func TestServer_StubError(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubError("namecheap.domains.getInfo", 2019166, "Domain not found")

	_, err := srv.Client().Domains.GetInfoWithContext(context.Background(), "missing.com")
	var apiErr *namecheap.APIError
	if assert.True(t, errors.As(err, &apiErr), "expected *APIError, got %v", err) {
		assert.Equal(t, 2019166, apiErr.Number)
		assert.Equal(t, "Domain not found", apiErr.Message)
	}
}

func TestServer_StubErrorMatchesSentinel(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubError("namecheap.domains.getInfo", 2019166)

	_, err := srv.Client().Domains.GetInfoWithContext(context.Background(), "missing.com")
	assert.True(t, errors.Is(err, namecheap.ErrDomainNotFound))
}

func TestServer_StubSequence(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	// Two distinct getInfo bodies, then the second repeats.
	srv.StubSequence("namecheap.domains.getInfo",
		fixtureWithName(t, "one.com"),
		fixtureWithName(t, "two.com"),
	)

	client := srv.Client()
	first, err := client.Domains.GetInfoWithContext(context.Background(), "x")
	assert.NoError(t, err)
	assert.Equal(t, "one.com", *first.DomainDNSGetListResult.DomainName)

	second, err := client.Domains.GetInfoWithContext(context.Background(), "x")
	assert.NoError(t, err)
	assert.Equal(t, "two.com", *second.DomainDNSGetListResult.DomainName)

	// Exhausted sequence: last entry repeats.
	third, err := client.Domains.GetInfoWithContext(context.Background(), "x")
	assert.NoError(t, err)
	assert.Equal(t, "two.com", *third.DomainDNSGetListResult.DomainName)

	assert.Len(t, srv.Calls("namecheap.domains.getInfo"), 3)
}

func TestServer_CallsCapturesParams(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.domains.getInfo", "domains_getInfo")

	_, _ = srv.Client().Domains.GetInfoWithContext(context.Background(), "captured.com")

	calls := srv.Calls("namecheap.domains.getInfo")
	if assert.Len(t, calls, 1) {
		assert.Equal(t, "captured.com", calls[0]["DomainName"])
		// Credentials the SDK injects are captured verbatim too.
		assert.Equal(t, "namecheap.domains.getInfo", calls[0]["Command"])
	}
}

func TestServer_UnstubbedCommandReturnsAPIError(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	// No stub registered.
	_, err := srv.Client().Domains.GetInfoWithContext(context.Background(), "x")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no stub registered")
}

func TestServer_ConcurrentUse(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.domains.getInfo", "domains_getInfo")
	client := srv.Client()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Domains.GetInfoWithContext(context.Background(), "example.com")
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.Len(t, srv.Calls("namecheap.domains.getInfo"), 16)
}

// fixtureWithName returns a minimal getInfo OK body whose DomainName is name,
// used to make StubSequence entries distinguishable.
func fixtureWithName(t *testing.T, name string) string {
	t.Helper()
	return `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.domains.getInfo">
		<DomainGetInfoResult DomainName="` + name + `" IsPremium="false" />
	</CommandResponse>
</ApiResponse>`
}
