package namecheap

import (
	"strings"
	"time"
)

// SSLService groups the namecheap.ssl.* API commands, covering the full
// certificate lifecycle: inventory (GetListWithContext, GetInfoWithContext,
// ParseCSRWithContext), activation (ActivateWithContext,
// GetApproverEmailListWithContext, ResendApproverEmailWithContext,
// EditDCVMethodWithContext) and the charge-bearing money operations
// (CreateWithContext, RenewWithContext, ReissueWithContext,
// PurchaseMoreSansWithContext, RevokeCertificateWithContext,
// ResendFulfillmentEmailWithContext).
//
// Keys and CSRs (non-goal). This SDK only transports the CSR string a consumer
// supplies; it never generates, parses locally, or stores private keys. Generate
// a key and CSR yourself (for example with crypto/x509 and encoding/pem) and pass
// the PEM-encoded CSR to ActivateWithContext / ReissueWithContext. Nothing in
// this package holds key material.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/
type SSLService service

// DCVMethod is a typed domain-control-validation (DCV) method selector used by
// ActivateWithContext and EditDCVMethodWithContext. Because it is a fixed enum,
// an invalid method cannot be expressed as a valid value; a bad string is
// rejected client-side by argument validation (see IsValid).
//
// Grounding and the documented gap. The Namecheap API doc
// (docs/namecheap-api-v2.md, ssl section) names a DCVMethod parameter for
// editDCVMethod (line 1087) but does NOT enumerate its accepted values, nor does
// the activate section (lines 895-912) document its DCV parameters at all. This
// SDK therefore does not fabricate a code table: it grounds the two non-email
// methods in the documented Namecheap DCV tokens sent on the wire
// (HTTP_CSR_HASH, CNAME_CSR_HASH) and, for email validation, sends the approver
// email address as the DCV value (the documented approver-email flow). The gap is
// flagged here and in README.md.
type DCVMethod string

const (
	// DCVMethodHTTP selects HTTP file-based validation. Its wire token is
	// "HTTP_CSR_HASH". No approver email is required.
	DCVMethodHTTP DCVMethod = "HTTP_CSR_HASH"
	// DCVMethodDNS selects DNS CNAME-based validation. Its wire token is
	// "CNAME_CSR_HASH". No approver email is required.
	DCVMethodDNS DCVMethod = "CNAME_CSR_HASH"
	// DCVMethodEmail selects email-based validation. It requires an approver email
	// address, which is what the SDK places on the wire as the DCV value (see
	// dcvWireValue); the "EMAIL" string itself is a client-side selector only.
	DCVMethodEmail DCVMethod = "EMAIL"
)

// IsValid reports whether m is one of the three supported DCV methods.
func (m DCVMethod) IsValid() bool {
	switch m {
	case DCVMethodHTTP, DCVMethodDNS, DCVMethodEmail:
		return true
	default:
		return false
	}
}

// SANEntry is one Subject Alternative Name (additional domain / host block) in a
// multi-domain (SAN) activation or DCV edit, carrying its own domain-control
// validation method. A single certificate can validate each SAN independently.
type SANEntry struct {
	// DomainName is the SAN / additional domain to validate. Required.
	DomainName string
	// DCVMethod is the DCV method for this SAN. Required.
	DCVMethod DCVMethod
	// ApproverEmail is the approver email address; required only when DCVMethod is
	// DCVMethodEmail.
	ApproverEmail string
}

// dcvWireValue returns the wire value that expresses method for a single domain.
// HTTP and DNS map to their documented tokens; email validation puts the approver
// email address on the wire (the documented approver-email flow).
func dcvWireValue(method DCVMethod, approverEmail string) string {
	switch method {
	case DCVMethodEmail:
		return approverEmail
	case DCVMethodHTTP, DCVMethodDNS:
		return string(method)
	default:
		return string(method)
	}
}

// dcvMissingFields returns the names (prefixed by label) of the fields method
// requires but that are missing, so a caller can list every offending field at
// once. Email DCV requires an approver email; HTTP and DNS DCV require no extra
// field. An unrecognised method reports label+"DCVMethod" so an invalid value is
// surfaced client-side before any network call.
func dcvMissingFields(label string, method DCVMethod, approverEmail string) []string {
	switch method {
	case DCVMethodHTTP, DCVMethodDNS:
		return nil
	case DCVMethodEmail:
		if strings.TrimSpace(approverEmail) == "" {
			return []string{label + "ApproverEmail"}
		}
		return nil
	default:
		return []string{label + "DCVMethod"}
	}
}

// CertStatus is a typed SSL certificate status. Unlike domain-transfer statuses,
// the Namecheap API doc DOES enumerate the certificate status vocabulary
// (docs/namecheap-api-v2.md lines 948-957), so these constants are grounded in
// that documented set rather than fabricated. The raw status string is always
// exposed verbatim on the response (see SSLGetInfoResult.Status); ClassifyStatus
// maps it, case-insensitively, onto one of these constants.
type CertStatus string

const (
	// CertStatusUnknown is used for an empty or unrecognised status string. It is
	// never treated as issued.
	CertStatusUnknown CertStatus = "UNKNOWN"
	// CertStatusActive means the certificate is activated and issued (doc line 952).
	// This is the state IsIssued reports true for.
	CertStatusActive CertStatus = "ACTIVE"
	// CertStatusNewPurchase is the initial state after purchase; ssl.activate is the
	// next step (doc line 953).
	CertStatusNewPurchase CertStatus = "NEWPURCHASE"
	// CertStatusNewRenewal is the initial state after a renewal purchase (doc line 954).
	CertStatusNewRenewal CertStatus = "NEWRENEWAL"
	// CertStatusPurchased means the certificate is activated and awaiting issuance
	// (doc line 955) — activated but not yet issued.
	CertStatusPurchased CertStatus = "PURCHASED"
	// CertStatusPurchaseError means an error occurred while processing the
	// certificate (doc line 956).
	CertStatusPurchaseError CertStatus = "PURCHASEERROR"
	// CertStatusCancelled means the certificate was cancelled (doc line 957).
	CertStatusCancelled CertStatus = "CANCELLED"
)

// ClassifyStatus maps a raw certificate status string onto a CertStatus using
// the documented status vocabulary (docs/namecheap-api-v2.md lines 948-957). It
// is case-insensitive and whitespace-trimming; an empty or unrecognised value
// yields CertStatusUnknown. No status code is invented.
func ClassifyStatus(status string) CertStatus {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "ACTIVE":
		return CertStatusActive
	case "NEWPURCHASE":
		return CertStatusNewPurchase
	case "NEWRENEWAL":
		return CertStatusNewRenewal
	case "PURCHASED":
		return CertStatusPurchased
	case "PURCHASEERROR":
		return CertStatusPurchaseError
	case "CANCELLED":
		return CertStatusCancelled
	default:
		return CertStatusUnknown
	}
}

// expiresWithin is the single, tested place that decides whether an expiry
// instant falls within a lead time of a reference "now". It returns true when
// expiry is at or before now+within (an inclusive boundary), so a certificate
// that expires exactly at the threshold — or that has already expired — counts as
// expiring soon. A zero expiry (never parsed) returns false. Comparison is on
// absolute instants, so it is timezone-safe regardless of how expiry or now were
// constructed.
func expiresWithin(expiry time.Time, within time.Duration, now time.Time) bool {
	if expiry.IsZero() {
		return false
	}
	return !expiry.After(now.Add(within))
}
