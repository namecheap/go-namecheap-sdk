package namecheap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDCVMethodIsValid confirms only the three documented methods are valid and
// an arbitrary string cannot masquerade as a valid method.
func TestDCVMethodIsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, DCVMethodHTTP.IsValid())
	assert.True(t, DCVMethodDNS.IsValid())
	assert.True(t, DCVMethodEmail.IsValid())
	assert.False(t, DCVMethod("").IsValid())
	assert.False(t, DCVMethod("FTP_HASH").IsValid())
}

// TestDCVWireValue confirms HTTP/DNS emit their tokens verbatim and email emits
// the approver address.
func TestDCVWireValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "HTTP_CSR_HASH", dcvWireValue(DCVMethodHTTP, ""))
	assert.Equal(t, "CNAME_CSR_HASH", dcvWireValue(DCVMethodDNS, ""))
	assert.Equal(t, "[email protected]", dcvWireValue(DCVMethodEmail, "[email protected]"))
}

// TestDCVMissingFieldsMatrix table-tests the per-method required-field rule: email
// requires an approver address, HTTP and DNS require nothing, and an unknown
// method surfaces DCVMethod as invalid. Every combination is covered.
func TestDCVMissingFieldsMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		method        DCVMethod
		approverEmail string
		want          []string
	}{
		{"http_ok", DCVMethodHTTP, "", nil},
		{"http_ignores_email", DCVMethodHTTP, "[email protected]", nil},
		{"dns_ok", DCVMethodDNS, "", nil},
		{"email_ok", DCVMethodEmail, "[email protected]", nil},
		{"email_missing_approver", DCVMethodEmail, "", []string{"ApproverEmail"}},
		{"email_blank_approver", DCVMethodEmail, "   ", []string{"ApproverEmail"}},
		{"unknown_method", DCVMethod("FTP"), "", []string{"DCVMethod"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, dcvMissingFields("", tc.method, tc.approverEmail))
		})
	}
}

// TestDCVMissingFieldsLabel confirms the label prefix is applied so per-SAN
// errors are attributable.
func TestDCVMissingFieldsLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"SANs[2].ApproverEmail"}, dcvMissingFields("SANs[2].", DCVMethodEmail, ""))
}

// TestClassifyStatus confirms every documented status maps to its constant,
// classification is case-insensitive and whitespace-tolerant, and an unknown or
// empty value yields CertStatusUnknown (no fabricated codes).
func TestClassifyStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]CertStatus{
		"Active":        CertStatusActive,
		"active":        CertStatusActive,
		"  ACTIVE  ":    CertStatusActive,
		"Newpurchase":   CertStatusNewPurchase,
		"NEWRENEWAL":    CertStatusNewRenewal,
		"Purchased":     CertStatusPurchased,
		"Purchaseerror": CertStatusPurchaseError,
		"Cancelled":     CertStatusCancelled,
		"":              CertStatusUnknown,
		"SomethingElse": CertStatusUnknown,
	}
	for raw, want := range cases {
		assert.Equalf(t, want, ClassifyStatus(raw), "status %q", raw)
	}
}

// TestExpiresWithinBoundary boundary-tests the single expiry-math helper: the
// threshold is inclusive, an already-expired instant counts, a zero expiry never
// counts, and the comparison is timezone-safe (same instant in another zone gives
// the same answer).
func TestExpiresWithinBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	within := 30 * 24 * time.Hour

	// Exactly at the threshold: inclusive -> true.
	assert.True(t, expiresWithin(now.Add(within), within, now))
	// One second past the threshold -> false.
	assert.False(t, expiresWithin(now.Add(within+time.Second), within, now))
	// Comfortably inside the window -> true.
	assert.True(t, expiresWithin(now.Add(within-time.Hour), within, now))
	// Already expired -> true (needs attention).
	assert.True(t, expiresWithin(now.Add(-time.Hour), within, now))
	// Zero expiry -> false.
	assert.False(t, expiresWithin(time.Time{}, within, now))

	// Timezone safety: the boundary instant expressed in a +2h zone is the same
	// instant, so the answer must not change.
	other := now.Add(within).In(time.FixedZone("UTC+2", 2*3600))
	assert.True(t, expiresWithin(other, within, now))
}

// TestGetInfoResultHelpers exercises IsIssued / IsExpiringSoon / CertStatus on the
// result, including nil-safety.
func TestGetInfoResultHelpers(t *testing.T) {
	t.Parallel()

	var nilResult *SSLGetInfoResult
	assert.Equal(t, CertStatusUnknown, nilResult.CertStatus())
	assert.False(t, nilResult.IsIssued())
	assert.False(t, nilResult.IsExpiringSoon(time.Hour))

	active := &SSLGetInfoResult{Status: "Active"}
	assert.True(t, active.IsIssued())
	assert.Equal(t, CertStatusActive, active.CertStatus())

	// Purchased is activated-but-awaiting-issuance: NOT issued.
	purchased := &SSLGetInfoResult{Status: "Purchased"}
	assert.False(t, purchased.IsIssued())

	// A far-future expiry is not expiring soon; a near one is.
	far := &SSLGetInfoResult{Expires: &DateTime{Time: time.Now().Add(365 * 24 * time.Hour)}}
	assert.False(t, far.IsExpiringSoon(30*24*time.Hour))
	near := &SSLGetInfoResult{Expires: &DateTime{Time: time.Now().Add(24 * time.Hour)}}
	assert.True(t, near.IsExpiringSoon(30*24*time.Hour))
	// A result with no expiry is never expiring soon.
	assert.False(t, (&SSLGetInfoResult{}).IsExpiringSoon(30*24*time.Hour))
}
