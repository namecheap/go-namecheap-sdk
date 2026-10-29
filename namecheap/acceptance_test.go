//go:build acceptance

// Package namecheap_test's acceptance suite exercises the real Namecheap API.
// It is excluded from the normal build by the "acceptance" build tag and only
// run via `make testacc`. It reads credentials from the environment and skips
// cleanly when they are absent, so it never fails a credential-less run.
// NAMECHEAP_USE_SANDBOX selects the endpoint: "true" targets the sandbox API,
// anything else targets production.
//
// Only read-only and reversible commands are exercised. Every mutation captures
// the prior state and restores it (defer), so reruns are idempotent and no
// account state is left changed.
//
// With -update-fixtures, the read-only responses are re-captured into
// ../namecheaptest/fixtures so CI can diff them against the committed corpus and
// surface server-shape drift as a reviewable diff. Fixture capture requires
// NAMECHEAP_USE_SANDBOX=true: the committed corpus is sandbox-shaped, and
// production responses must never overwrite it.
package namecheap_test

import (
	"context"
	"flag"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	envAPIUser    = "NAMECHEAP_API_USER"
	envAPIKey     = "NAMECHEAP_API_KEY"
	envClientIP   = "NAMECHEAP_CLIENT_IP"
	envUserName   = "NAMECHEAP_USER_NAME"
	envUseSandbox = "NAMECHEAP_USE_SANDBOX"
	// envDomain names a dedicated, disposable test domain in the target account
	// used for the reversible DNS round-trip. The DNS test skips when it is unset.
	envDomain = "NAMECHEAP_TEST_DOMAIN"
	// envRenewDomain opts in to TestAcc_DomainsRenew, the one test that is
	// neither read-only nor reversible: it renews the named domain for a year
	// and spends account funds (sandbox test funds when NAMECHEAP_USE_SANDBOX
	// is true). It exists to prove the DateTime parser against the real
	// domains.renew response and is never set in CI.
	envRenewDomain = "NAMECHEAP_ACC_RENEW_DOMAIN"
)

// dateOnlyPadded is the form domains.getList and domains.getInfo have always
// used, confirmed on production on 2026-10-23. A change here is server-side
// drift worth a look.
var dateOnlyPadded = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)

// assertLiveDate checks that a live date field parsed and that its raw text
// still has the shape this endpoint has always used.
func assertLiveDate(t *testing.T, field string, dt *namecheap.DateTime, shape *regexp.Regexp) {
	t.Helper()
	if dt == nil || dt.IsZero() {
		t.Errorf("%s = %v, want a parsed date", field, dt)
		return
	}
	if err := dt.ParseError(); err != nil {
		t.Errorf("%s: %v", field, err)
	}
	if !shape.MatchString(dt.Raw) {
		t.Errorf("%s raw text = %q, want shape %s (server-side format drift?)", field, dt.Raw, shape)
	}
}

// assertLiveDateText is assertLiveDate for fields the SDK exposes as raw
// strings rather than DateTime.
func assertLiveDateText(t *testing.T, field string, raw *string, shape *regexp.Regexp) {
	t.Helper()
	if raw == nil {
		t.Errorf("%s is missing, want a date", field)
		return
	}
	if !shape.MatchString(*raw) {
		t.Errorf("%s raw text = %q, want shape %s (server-side format drift?)", field, *raw, shape)
	}
}

// liveClient builds a client for the live API from environment credentials,
// skipping the test when any required variable is missing. The endpoint is
// production unless NAMECHEAP_USE_SANDBOX is true.
func liveClient(t *testing.T) *namecheap.Client {
	t.Helper()
	apiUser := os.Getenv(envAPIUser)
	apiKey := os.Getenv(envAPIKey)
	clientIP := os.Getenv(envClientIP)
	if apiUser == "" || apiKey == "" || clientIP == "" {
		t.Skipf("live API credentials not set; skipping (need %s, %s, %s[, %s, %s])",
			envAPIUser, envAPIKey, envClientIP, envUserName, envDomain)
	}
	userName := os.Getenv(envUserName)
	if userName == "" {
		userName = apiUser
	}
	useSandbox, _ := strconv.ParseBool(os.Getenv(envUseSandbox))
	return namecheap.NewClient(&namecheap.ClientOptions{
		UserName:   userName,
		ApiUser:    apiUser,
		ApiKey:     apiKey,
		ClientIp:   clientIP,
		UseSandbox: useSandbox,
		// Conservative pacing: the live API throttles aggressively, and the
		// acceptance account is shared with other CI runs.
		RateLimit: &namecheap.RateLimitOptions{
			PerMinute:     10,
			MaxConcurrent: 1,
		},
		Retry: &namecheap.RetryOptions{
			MaxAttempts: 5,
			BaseDelay:   2 * time.Second,
			MaxDelay:    30 * time.Second,
		},
	})
}

// ctx returns a per-test context with a generous timeout for the live API.
// It must accommodate the full retry budget (RetryOptions.MaxElapsed defaults
// to 2 minutes), not just a single round trip.
func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return c
}

func TestAcc_DomainsCheck(t *testing.T) {
	const freeCandidate = "example-that-should-be-free-12345.com"
	client := liveClient(t)

	// Check the free candidate plus, when configured, the account's own test
	// domain — the one name guaranteed to be registered in this registry.
	domains := []string{freeCandidate}
	testDomain := os.Getenv(envDomain)
	if testDomain != "" {
		domains = append(domains, testDomain)
	}

	resp, err := client.Domains.CheckWithContext(ctx(t), domains...)
	if err != nil {
		t.Fatalf("domains.check: %v", err)
	}
	if resp == nil || resp.DomainCheckResults == nil {
		t.Fatal("domains.check: empty result")
	}

	availability := map[string]bool{}
	for _, r := range *resp.DomainCheckResults {
		if r.Domain == nil || r.IsAvailable == nil {
			t.Fatalf("domains.check: result missing Domain or Available attr: %+v", r)
		}
		availability[strings.ToLower(*r.Domain)] = *r.IsAvailable
	}
	if got, ok := availability[freeCandidate]; !ok || !got {
		t.Errorf("domains.check(%q) available = %v (present=%v), want true", freeCandidate, got, ok)
	}
	if testDomain != "" {
		if got, ok := availability[strings.ToLower(testDomain)]; !ok || got {
			t.Errorf("domains.check(%q) available = %v (present=%v), want false (registered test domain)", testDomain, got, ok)
		}
	}

	captureFixture(t, client, "domains_check", map[string]string{
		"Command":    "namecheap.domains.check",
		"DomainList": freeCandidate,
	})
}

func TestAcc_DomainsGetList(t *testing.T) {
	client := liveClient(t)
	args := &namecheap.DomainsGetListArgs{
		PageSize: namecheap.Int(10),
	}
	// When a test domain is configured, filter for it so the assertion is
	// independent of account size and listing order.
	testDomain := os.Getenv(envDomain)
	if testDomain != "" {
		args.SearchTerm = namecheap.String(testDomain)
	}

	resp, err := client.Domains.GetListWithContext(ctx(t), args)
	if err != nil {
		t.Fatalf("domains.getList: %v", err)
	}
	if resp == nil || resp.Domains == nil {
		t.Fatal("domains.getList: empty result")
	}

	if testDomain != "" {
		var found *namecheap.Domain
		for i, d := range *resp.Domains {
			if d.Name != nil && strings.EqualFold(*d.Name, testDomain) {
				found = &(*resp.Domains)[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("domains.getList(SearchTerm=%q) did not return the test domain", testDomain)
		}
		if found.ID == nil || *found.ID == "" {
			t.Errorf("domains.getList: test domain ID = %v, want non-empty", found.ID)
		}
		assertLiveDate(t, "domains.getList Created", found.Created, dateOnlyPadded)
		assertLiveDate(t, "domains.getList Expires", found.Expires, dateOnlyPadded)
	}

	captureFixture(t, client, "domains_getList", map[string]string{
		"Command":  "namecheap.domains.getList",
		"PageSize": "10",
	})
}

func TestAcc_DomainsGetInfo(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := liveClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping domains.getInfo", envDomain)
	}
	resp, err := client.Domains.GetInfoWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("domains.getInfo: %v", err)
	}
	if resp == nil {
		t.Fatal("domains.getInfo: nil command response")
	}
	result := resp.Result()
	if result == nil {
		t.Fatal("domains.getInfo: nil result")
	}

	// A decode abort surfaces as err above; these assertions catch the other
	// failure mode — a struct-tag mismatch leaving whole sections silently nil.
	if result.DomainName == nil || !strings.EqualFold(*result.DomainName, domain) {
		t.Errorf("getInfo DomainName = %v, want %q", result.DomainName, domain)
	}
	if result.ID == nil || *result.ID <= 0 {
		t.Errorf("getInfo ID = %v, want a positive integer", result.ID)
	}
	// Status is not present in every response; when present it must be non-empty.
	if result.Status != nil && *result.Status == "" {
		t.Error("getInfo Status is present but empty")
	}

	if result.DomainDetails == nil {
		t.Error("getInfo DomainDetails is nil, want populated")
	} else {
		assertLiveDate(t, "getInfo DomainDetails.CreatedDate", result.DomainDetails.CreatedDate, dateOnlyPadded)
		assertLiveDate(t, "getInfo DomainDetails.ExpiredDate", result.DomainDetails.ExpiredDate, dateOnlyPadded)
	}

	if result.WhoisGuard == nil {
		t.Error("getInfo WhoisGuard is nil, want populated")
	} else if result.WhoisGuard.Enabled == nil {
		t.Error("getInfo WhoisGuard.Enabled is nil, want tri-state value")
	} else if got := *result.WhoisGuard.Enabled; got != "True" && got != "False" && got != "NotAlloted" {
		t.Errorf("getInfo WhoisGuard.Enabled = %q, want True/False/NotAlloted", got)
	}
	// An allotted subscription (True or False) carries its own expiry, formatted
	// independently of whoisguard.getList; NotAlloted has none.
	if wg := result.WhoisGuard; wg != nil && wg.Enabled != nil && *wg.Enabled != "NotAlloted" {
		if wg.ExpiredDate != nil {
			t.Logf("getInfo Whoisguard.ExpiredDate raw text %q", wg.ExpiredDate.Raw)
		}
		assertLiveDate(t, "getInfo Whoisguard.ExpiredDate", wg.ExpiredDate, dateOnlyPadded)
	}

	if result.DnsDetails == nil {
		t.Error("getInfo DnsDetails is nil, want populated")
	} else {
		if result.DnsDetails.ProviderType == nil || *result.DnsDetails.ProviderType == "" {
			t.Errorf("getInfo DnsDetails.ProviderType = %v, want non-empty", result.DnsDetails.ProviderType)
		}
		if result.DnsDetails.Nameservers == nil || len(*result.DnsDetails.Nameservers) == 0 {
			t.Error("getInfo DnsDetails.Nameservers is empty, want at least one")
		}
	}

	if result.ModificationRights == nil {
		t.Error("getInfo ModificationRights is nil, want populated")
	}

	captureFixture(t, client, "domains_getInfo", map[string]string{
		"Command":    "namecheap.domains.getInfo",
		"DomainName": domain,
		"HostName":   domain,
	})
}

// TestAcc_DomainPrivacyGetList is the live-format check for whoisguard.getList
// dates. Created and Expires are raw strings in the SDK, so a format change
// would never fail decoding; only this check notices it. The raw values are
// logged as evidence of what the API returns.
func TestAcc_DomainPrivacyGetList(t *testing.T) {
	client := liveClient(t)
	resp, err := client.DomainPrivacy.GetListWithContext(ctx(t), &namecheap.DomainPrivacyGetListArgs{
		PageSize: namecheap.Int(10),
	})
	if err != nil {
		t.Fatalf("whoisguard.getList: %v", err)
	}
	if resp == nil || resp.DomainPrivacyList == nil || len(*resp.DomainPrivacyList) == 0 {
		t.Fatal("whoisguard.getList: no subscriptions returned, want at least one to check dates on")
	}

	for _, e := range *resp.DomainPrivacyList {
		field := "whoisguard.getList ID=<nil>"
		if e.ID != nil {
			field = "whoisguard.getList ID=" + strconv.Itoa(*e.ID)
		}
		t.Logf("%s Status=%v Created=%v Expires=%v", field, quoteAttr(e.Status), quoteAttr(e.Created), quoteAttr(e.Expires))
		assertLiveDateText(t, field+" Created", e.Created, dateOnlyPadded)
		assertLiveDateText(t, field+" Expires", e.Expires, dateOnlyPadded)
	}

	captureFixture(t, client, "whoisguard_getlist", map[string]string{
		"Command":  "namecheap.whoisguard.getlist",
		"PageSize": "10",
	})
}

// quoteAttr renders an optional attribute for logs, quoting it so an empty value
// stays visible.
func quoteAttr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return strconv.Quote(*s)
}

// TestAcc_DomainsRenew proves the DateTime parser against a real domains.renew
// response, the one endpoint whose date carries a time component in a
// server-dependent format (captured 2026-10-26: "12/14/2034 1:01:07\u202fPM").
// It is opt-in through NAMECHEAP_ACC_RENEW_DOMAIN because it spends funds and
// adds a year to the domain; a decode failure here is exactly the
// charge-then-error outcome this parser exists to prevent.
func TestAcc_DomainsRenew(t *testing.T) {
	domain := os.Getenv(envRenewDomain)
	client := liveClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping domains.renew (spends funds, not reversible)", envRenewDomain)
	}

	before, err := client.Domains.GetInfoWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("domains.getInfo before renew: %v", err)
	}
	if before.Result() == nil || before.Result().DomainDetails == nil || before.Result().DomainDetails.ExpiredDate == nil {
		t.Fatal("domains.getInfo before renew: no ExpiredDate to compare against")
	}
	expiryBefore := before.Result().DomainDetails.ExpiredDate.Time

	resp, err := client.Domains.RenewWithContext(ctx(t), &namecheap.DomainsRenewArgs{DomainName: domain, Years: 1})
	if err != nil {
		t.Fatalf("domains.renew: %v (a successful charge may have been made)", err)
	}
	result := resp.DomainRenewResult
	if result == nil || result.Renew == nil || !*result.Renew {
		t.Fatalf("domains.renew: Renew = %v, want true", result)
	}
	if result.DomainDetails == nil || result.DomainDetails.ExpiredDate == nil {
		t.Fatal("domains.renew: DomainDetails.ExpiredDate missing")
	}
	got := result.DomainDetails.ExpiredDate
	t.Logf("domains.renew ExpiredDate raw text %q (bytes % x)", got.Raw, []byte(got.Raw))
	if err := got.ParseError(); err != nil {
		t.Errorf("domains.renew ExpiredDate: %v", err)
	}
	if got.Year() != expiryBefore.Year()+1 {
		t.Errorf("domains.renew ExpiredDate = %v, want year %d (expiry before: %v)", got.Time, expiryBefore.Year()+1, expiryBefore)
	}
}

func TestAcc_UsersGetBalances(t *testing.T) {
	client := liveClient(t)
	resp, err := client.Users.GetBalancesWithContext(ctx(t))
	if err != nil {
		t.Fatalf("users.getBalances: %v", err)
	}
	if resp == nil || resp.UserGetBalancesResult == nil {
		t.Fatal("users.getBalances: empty result")
	}
	if resp.UserGetBalancesResult.Currency == "" {
		t.Error("users.getBalances: Currency is empty, want e.g. USD")
	}
	captureFixture(t, client, "users_getBalances", map[string]string{
		"Command": "namecheap.users.getBalances",
	})
}

func TestAcc_UsersGetPricing(t *testing.T) {
	client := liveClient(t)
	resp, err := client.Users.GetPricingWithContext(ctx(t), &namecheap.UsersGetPricingArgs{
		ProductType: namecheap.String("DOMAIN"),
		ActionName:  namecheap.String("REGISTER"),
		ProductName: namecheap.String("com"),
	})
	if err != nil {
		t.Fatalf("users.getPricing: %v", err)
	}
	if resp == nil || resp.UserGetPricingResult == nil {
		t.Fatal("users.getPricing: empty result")
	}
	// The narrowed query (DOMAIN/REGISTER/com) must yield at least one positive
	// price tier somewhere in the type -> category -> product -> price tree.
	positiveTier := false
	for _, pt := range resp.UserGetPricingResult.ProductTypes {
		for _, cat := range pt.ProductCategories {
			for _, prod := range cat.Products {
				for _, price := range prod.Prices {
					if price.Price.IsPositive() {
						positiveTier = true
					}
				}
			}
		}
	}
	if !positiveTier {
		t.Error("users.getPricing(DOMAIN/REGISTER/com): no positive price tier in response")
	}
	captureFixture(t, client, "users_getPricing", map[string]string{
		"Command":     "namecheap.users.getPricing",
		"ProductType": "DOMAIN",
		"ActionName":  "REGISTER",
		"ProductName": "com",
	})
}

// TestAcc_DNSRoundTrip exercises the get -> set -> restore reversible flow on
// a dedicated test domain. It captures the current host records, writes them back
// unchanged, and restores them via defer, leaving the zone exactly as found.
func TestAcc_DNSRoundTrip(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := liveClient(t)
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

	// Read back and verify the zone matches what was written — the round trip
	// must be lossless, not merely error-free.
	reread, err := client.DomainsDNS.GetHostsWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("dns.getHosts read-back: %v", err)
	}
	after := toSetRecords(reread)
	if len(after) != len(original) {
		t.Fatalf("dns round-trip: read back %d records, wrote %d", len(after), len(original))
	}
	want := map[string]int{}
	for _, r := range original {
		want[recordKey(r)]++
	}
	for _, r := range after {
		if want[recordKey(r)] == 0 {
			t.Errorf("dns round-trip: unexpected record after write: %s", recordKey(r))
			continue
		}
		want[recordKey(r)]--
	}
	for k, n := range want {
		if n > 0 {
			t.Errorf("dns round-trip: record lost by write: %s", k)
		}
	}
}

// recordKey canonicalizes a host record for set comparison, ignoring order.
func recordKey(r namecheap.DomainsDNSHostRecord) string {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	ttl := ""
	if r.TTL != nil {
		ttl = strconv.Itoa(*r.TTL)
	}
	return strings.ToLower(deref(r.HostName)) + "|" + deref(r.RecordType) + "|" + deref(r.Address) + "|" + ttl
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
