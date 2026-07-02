package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLReissueResponse is the raw envelope for namecheap.ssl.reissue.
type SSLReissueResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLReissueCommandResponse `xml:"CommandResponse"`
}

// SSLReissueCommandResponse wraps the reissue result.
type SSLReissueCommandResponse struct {
	SSLReissueResult *SSLReissueResult `xml:"SSLReissueResult"`
}

// SSLReissueResult is the outcome of a reissue. The doc's reissue section
// (docs/namecheap-api-v2.md lines 988-1005) does not tabulate the response fields;
// like activate, a reissue restarts domain-control validation, so the HTTP-DCV
// file details are surfaced alongside the success flag.
type SSLReissueResult struct {
	// ID is the unique integer identifying the certificate.
	ID *int `xml:"ID,attr"`
	// IsSuccess reports whether the reissue request was accepted.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// HTTPDCValidation carries the HTTP DCV file details when HTTP validation is set.
	HTTPDCValidation *SSLHTTPDCValidation `xml:"HttpDCValidation"`
}

// SSLReissueArgs are the arguments for ReissueWithContext. Field names and
// required status follow the reissue request table in docs/namecheap-api-v2.md
// (lines 996-1002).
type SSLReissueArgs struct {
	// CertificateID is the certificate to reissue. Required.
	CertificateID int
	// CSR is the PEM-encoded Certificate Signing Request. Required. Transported
	// verbatim; no key material is read or stored.
	CSR string
	// AdminEmailAddress is the admin email; it cannot be changed from the initial
	// activation. Optional.
	AdminEmailAddress string
	// WebServerType is the server software type (apacheopenssl, nginx, iis, ...).
	// Optional.
	WebServerType string
	// UniqueValue is an optional unique identifier for the reissue request.
	UniqueValue string
}

// ReissueWithContext reissues an SSL certificate.
//
// It is classified as a non-idempotent lifecycle call (same treatment as the
// SSL money operations): on an ambiguous transport or server-side failure the SDK
// does NOT retry, since a reissue can re-enter billing/validation flows. Only the
// pre-execution HTTP 405 rate-limit signal is retried. Reconcile ambiguous
// failures via GetInfoWithContext.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/reissue/
func (ss *SSLService) ReissueWithContext(ctx context.Context, args *SSLReissueArgs) (*SSLReissueCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response SSLReissueResponse
	// idempotent=false: never retry on an ambiguous error (see doc comment above).
	_, err := ss.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing required field at once.
func (a *SSLReissueArgs) validate() error {
	missing := make([]string, 0, 2)
	if a.CertificateID < 1 {
		missing = append(missing, "CertificateID")
	}
	if a.CSR == "" {
		missing = append(missing, "CSR")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map.
func (a *SSLReissueArgs) params() map[string]string {
	params := map[string]string{
		"Command":       "namecheap.ssl.reissue",
		"CertificateID": strconv.Itoa(a.CertificateID),
		"CSR":           a.CSR,
	}
	setIfNotEmpty(params, "AdminEmailAddress", a.AdminEmailAddress)
	setIfNotEmpty(params, "WebServerType", a.WebServerType)
	setIfNotEmpty(params, "UniqueValue", a.UniqueValue)
	return params
}
