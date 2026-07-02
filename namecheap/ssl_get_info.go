package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
	"time"
)

// SSLGetInfoResponse is the raw envelope for namecheap.ssl.getInfo.
type SSLGetInfoResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLGetInfoCommandResponse `xml:"CommandResponse"`
}

// SSLGetInfoCommandResponse wraps the getInfo result.
type SSLGetInfoCommandResponse struct {
	SSLGetInfoResult *SSLGetInfoResult `xml:"SSLGetInfoResult"`
}

// SSLGetInfoResult is the detail of a single certificate. The doc's getInfo
// section (docs/namecheap-api-v2.md lines 934-957) enumerates the certificate
// Status vocabulary but not the full response field set; the fields below follow
// the documented status vocabulary plus the identity/expiry attributes the
// certificate carries. The raw Status string is exposed verbatim (classify it
// with CertStatus / ClassifyStatus).
type SSLGetInfoResult struct {
	// CertificateID is the unique integer identifying the certificate.
	CertificateID *int `xml:"CertificateID,attr"`
	// Status is the raw certificate status string, one of the documented values
	// (doc lines 948-957). Classify it with the CertStatus helper.
	Status string `xml:"Status,attr"`
	// Type is the certificate product type.
	Type *string `xml:"Type,attr"`
	// CommonName is the primary host the certificate is issued for.
	CommonName *string `xml:"CommonName,attr"`
	// Provider is the issuing certificate authority / provider name.
	Provider *string `xml:"Provider,attr"`
	// IssuedOn is the date the certificate was issued (MM/DD/YYYY), when present.
	IssuedOn *DateTime `xml:"IssuedOn,attr"`
	// Expires is the date the certificate expires (MM/DD/YYYY), when present. It is
	// the input to IsExpiringSoon.
	Expires *DateTime `xml:"Expires,attr"`
}

// CertStatus classifies the result's raw Status string into a typed CertStatus
// (see ClassifyStatus). It returns CertStatusUnknown for a nil result or an empty
// / unrecognised status.
func (r *SSLGetInfoResult) CertStatus() CertStatus {
	if r == nil {
		return CertStatusUnknown
	}
	return ClassifyStatus(r.Status)
}

// IsIssued reports whether the certificate is issued and usable, i.e. its status
// classifies to CertStatusActive (the documented "Active" state, doc line 952).
// The documented "Purchased" state means activated-but-awaiting-issuance and is
// deliberately NOT reported as issued.
func (r *SSLGetInfoResult) IsIssued() bool {
	return r.CertStatus() == CertStatusActive
}

// IsExpiringSoon reports whether the certificate expires within the given lead
// time from now. The boundary is inclusive (an expiry exactly at now+within
// counts), an already-expired certificate returns true, and a missing/zero expiry
// returns false. All expiry math lives in the tested expiresWithin helper and is
// timezone-safe. A nil result returns false.
func (r *SSLGetInfoResult) IsExpiringSoon(within time.Duration) bool {
	if r == nil || r.Expires == nil {
		return false
	}
	return expiresWithin(r.Expires.Time, within, time.Now())
}

// GetInfoWithContext retrieves detailed information about the certificate
// identified by certificateID. It is an idempotent read and retries on transient
// failures.
//
// returnCertificate, when non-empty, is sent as the Returncertificate flag and
// returnType (e.g. "Individual" or "PKCS7") as Returntype (doc lines 945-946);
// pass "" for both to omit them.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/get-info/
func (ss *SSLService) GetInfoWithContext(ctx context.Context, certificateID int, returnCertificate, returnType string) (*SSLGetInfoCommandResponse, error) {
	if certificateID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"CertificateID"}, Reason: "CertificateID must be a positive integer"}
	}

	params := map[string]string{
		"Command":       "namecheap.ssl.getInfo",
		"CertificateID": strconv.Itoa(certificateID),
	}
	setIfNotEmpty(params, "Returncertificate", returnCertificate)
	setIfNotEmpty(params, "Returntype", returnType)

	var response SSLGetInfoResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
