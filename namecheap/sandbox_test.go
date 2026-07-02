//go:build sandbox

// Package namecheap_test's sandbox suite exercises the real Namecheap sandbox
// API. It is excluded from the normal build by the "sandbox" build tag and only
// compiled/run via `make test-sandbox` (go test -tags sandbox). It reads
// credentials from the environment and skips cleanly when they are absent, so it
// never fails a credential-less run.
//
// Only read-only and reversible commands are exercised. Every mutation captures
// the prior state and restores it (defer), so reruns are idempotent and no
// sandbox state is left changed. Production is never touched: the client is built
// with UseSandbox: true.
//
// With -update-fixtures, the read-only responses are re-captured into
// ../namecheaptest/fixtures so CI can diff them against the committed corpus and
// surface server-shape drift as a reviewable diff.
package namecheap_test

import (
	"context"
	"flag"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
)

// updateFixtures, when set, re-captures the read-only sandbox responses into the
// committed fixture corpus so drift surfaces as a git diff.
var updateFixtures = flag.Bool("update-fixtures", false,
	"re-capture read-only sandbox responses into ../namecheaptest/fixtures")

const (
	envAPIUser  = "NAMECHEAP_SANDBOX_APIUSER"
	envAPIKey   = "NAMECHEAP_SANDBOX_APIKEY"
	envClientIP = "NAMECHEAP_SANDBOX_CLIENTIP"
	envUserName = "NAMECHEAP_SANDBOX_USERNAME"
	// envDomain names a dedicated, disposable test domain in the sandbox account
	// used for the reversible DNS round-trip. The DNS test skips when it is unset.
	envDomain = "NAMECHEAP_SANDBOX_DOMAIN"
)

// sandboxClient builds a sandbox-pointed client from environment credentials,
// skipping the test when any required variable is missing.
func sandboxClient(t *testing.T) *namecheap.Client {
	t.Helper()
	apiUser := os.Getenv(envAPIUser)
	apiKey := os.Getenv(envAPIKey)
	clientIP := os.Getenv(envClientIP)
	if apiUser == "" || apiKey == "" || clientIP == "" {
		t.Skipf("sandbox credentials not set; skipping (need %s, %s, %s[, %s, %s])",
			envAPIUser, envAPIKey, envClientIP, envUserName, envDomain)
	}
	userName := os.Getenv(envUserName)
	if userName == "" {
		userName = apiUser
	}
	return namecheap.NewClient(&namecheap.ClientOptions{
		UserName:   userName,
		ApiUser:    apiUser,
		ApiKey:     apiKey,
		ClientIp:   clientIP,
		UseSandbox: true,
	})
}

// ctx returns a per-test context with a generous timeout for the live API.
func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestSandbox_DomainsCheck(t *testing.T) {
	client := sandboxClient(t)
	resp, err := client.Domains.CheckWithContext(ctx(t), "example-that-should-be-free-12345.com")
	if err != nil {
		t.Fatalf("domains.check: %v", err)
	}
	if resp == nil || resp.DomainCheckResults == nil {
		t.Fatal("domains.check: empty result")
	}
	captureFixture(t, client, "domains_check", map[string]string{
		"Command":    "namecheap.domains.check",
		"DomainList": "example-that-should-be-free-12345.com",
	})
}

func TestSandbox_DomainsGetList(t *testing.T) {
	client := sandboxClient(t)
	_, err := client.Domains.GetListWithContext(ctx(t), &namecheap.DomainsGetListArgs{
		PageSize: namecheap.Int(10),
	})
	if err != nil {
		t.Fatalf("domains.getList: %v", err)
	}
	captureFixture(t, client, "domains_getList", map[string]string{
		"Command":  "namecheap.domains.getList",
		"PageSize": "10",
	})
}

func TestSandbox_DomainsGetInfo(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := sandboxClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping domains.getInfo", envDomain)
	}
	if _, err := client.Domains.GetInfoWithContext(ctx(t), domain); err != nil {
		t.Fatalf("domains.getInfo: %v", err)
	}
	captureFixture(t, client, "domains_getInfo", map[string]string{
		"Command":    "namecheap.domains.getInfo",
		"DomainName": domain,
		"HostName":   domain,
	})
}

func TestSandbox_UsersGetBalances(t *testing.T) {
	client := sandboxClient(t)
	if _, err := client.Users.GetBalancesWithContext(ctx(t)); err != nil {
		t.Fatalf("users.getBalances: %v", err)
	}
	captureFixture(t, client, "users_getBalances", map[string]string{
		"Command": "namecheap.users.getBalances",
	})
}

func TestSandbox_UsersGetPricing(t *testing.T) {
	client := sandboxClient(t)
	_, err := client.Users.GetPricingWithContext(ctx(t), &namecheap.UsersGetPricingArgs{
		ProductType: namecheap.String("DOMAIN"),
		ActionName:  namecheap.String("REGISTER"),
		ProductName: namecheap.String("com"),
	})
	if err != nil {
		t.Fatalf("users.getPricing: %v", err)
	}
	captureFixture(t, client, "users_getPricing", map[string]string{
		"Command":     "namecheap.users.getPricing",
		"ProductType": "DOMAIN",
		"ActionName":  "REGISTER",
		"ProductName": "com",
	})
}

// TestSandbox_DNSRoundTrip exercises the get -> set -> restore reversible flow on
// a dedicated test domain. It captures the current host records, writes them back
// unchanged, and restores them via defer, leaving the zone exactly as found.
func TestSandbox_DNSRoundTrip(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := sandboxClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping DNS round-trip", envDomain)
	}

	got, err := client.DomainsDNS.GetHostsWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("dns.getHosts: %v", err)
	}
	captureFixture(t, client, "domains_dns_getHosts", map[string]string{
		"Command": "namecheap.domains.dns.getHosts",
	})

	original := toSetRecords(got)

	// Always restore the captured state, even if the round-trip write fails.
	defer func() {
		if _, rerr := client.DomainsDNS.SetHostsWithContext(context.Background(), &namecheap.DomainsDNSSetHostsArgs{
			Domain:  namecheap.String(domain),
			Records: &original,
		}); rerr != nil {
			t.Errorf("dns.setHosts restore failed (zone may be modified): %v", rerr)
		}
	}()

	// Round-trip write: set the exact records we read, a no-op net change that
	// still exercises setHosts end-to-end.
	if _, err := client.DomainsDNS.SetHostsWithContext(ctx(t), &namecheap.DomainsDNSSetHostsArgs{
		Domain:  namecheap.String(domain),
		Records: &original,
	}); err != nil {
		t.Fatalf("dns.setHosts round-trip: %v", err)
	}
}

// toSetRecords converts the detailed host records returned by getHosts into the
// setHosts input shape.
func toSetRecords(resp *namecheap.DomainsDNSGetHostsCommandResponse) []namecheap.DomainsDNSHostRecord {
	if resp == nil || resp.DomainDNSGetHostsResult == nil || resp.DomainDNSGetHostsResult.Hosts == nil {
		return nil
	}
	hosts := *resp.DomainDNSGetHostsResult.Hosts
	records := make([]namecheap.DomainsDNSHostRecord, 0, len(hosts))
	for _, h := range hosts {
		rec := namecheap.DomainsDNSHostRecord{
			HostName:   h.Name,
			RecordType: h.Type,
			Address:    h.Address,
			TTL:        h.TTL,
		}
		if h.MXPref != nil {
			rec.MXPref = namecheap.UInt8(uint8(*h.MXPref))
		}
		records = append(records, rec)
	}
	return records
}

// captureFixture re-captures a raw sandbox response into the committed fixture
// corpus when -update-fixtures is set. It performs a direct POST (bypassing the
// SDK's response decoding) to obtain the verbatim XML body. It is a no-op
// otherwise.
func captureFixture(t *testing.T, c *namecheap.Client, short string, params map[string]string) {
	t.Helper()
	if !*updateFixtures {
		return
	}
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("Username", c.ClientOptions.UserName)
	form.Set("ApiUser", c.ClientOptions.ApiUser)
	form.Set("ApiKey", c.ClientOptions.ApiKey)
	form.Set("ClientIp", c.ClientOptions.ClientIp)

	resp, err := http.Post(c.BaseURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("capture %s: %v", short, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("capture %s: read: %v", short, err)
	}
	path := filepath.Join("..", "namecheaptest", "fixtures", short+".xml")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("capture %s: write: %v", short, err)
	}
	t.Logf("captured fixture %s (%d bytes)", short, len(body))
}
