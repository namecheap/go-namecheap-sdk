package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// SSLActivateResponse is the raw envelope for namecheap.ssl.activate.
type SSLActivateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLActivateCommandResponse `xml:"CommandResponse"`
}

// SSLActivateCommandResponse wraps the activate result.
type SSLActivateCommandResponse struct {
	SSLActivateResult *SSLActivateResult `xml:"SSLActivateResult"`
}

// SSLActivateResult is the outcome of an activation. The doc's activate section
// (docs/namecheap-api-v2.md lines 895-912) does not tabulate the response fields;
// these follow the fields a certificate activation returns, with the HTTP-DCV
// file details surfaced for the HTTP validation flow.
type SSLActivateResult struct {
	// ID is the unique integer identifying the certificate.
	ID *int `xml:"ID,attr"`
	// IsSuccess reports whether the activation request was accepted.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// HTTPDCValidationFileName is the file name for the HTTP DCV text file, when
	// HTTP validation is in use.
	HTTPDCValidationFileName *string `xml:"HttpDCValidation>FileName"`
	// HTTPDCValidationFileContent is the content for the HTTP DCV text file, when
	// HTTP validation is in use.
	HTTPDCValidationFileContent *string `xml:"HttpDCValidation>FileContent"`
}

// SSLActivateArgs are the arguments for ActivateWithContext.
//
// Documented fields (doc lines 903-910): CertificateID, CSR, AdminEmailAddress,
// WebServerType and UniqueValue. The doc does NOT enumerate the activate DCV or
// multi-SAN parameters, so DCVMethod / ApproverEmail (primary domain) and SANs
// (additional domains) follow the documented Namecheap multi-domain activation
// contract; the gap is flagged on DCVMethod and in README.md.
type SSLActivateArgs struct {
	// CertificateID is the certificate to activate. Required.
	CertificateID int
	// CSR is the PEM-encoded Certificate Signing Request. Required. The SDK
	// transports it verbatim and never inspects or stores key material.
	CSR string
	// AdminEmailAddress is where the signed certificate file is sent. Required.
	AdminEmailAddress string
	// WebServerType is the server software type (apacheopenssl, nginx, iis, ...).
	// Optional.
	WebServerType string
	// UniqueValue is an optional unique identifier for the issue/reissue request.
	UniqueValue string

	// DCVMethod is the domain-control-validation method for the primary common
	// name. Optional; when set it is validated per-method (email requires
	// ApproverEmail).
	DCVMethod DCVMethod
	// ApproverEmail is the primary domain's approver email; required only when
	// DCVMethod is DCVMethodEmail.
	ApproverEmail string

	// SANs are the additional domains (host blocks) for a multi-domain (SAN)
	// certificate, each with its own DCV method. Optional; each entry is validated
	// and serialized as an indexed parameter block.
	SANs []SANEntry
}

// ActivateWithContext activates a newly purchased SSL certificate.
//
// It is idempotent from a billing standpoint (activation is not charge-bearing)
// and retries on transient failures. Arguments are validated client-side first,
// reporting every missing field at once via *InvalidArgumentsError — activation
// failures are slow and opaque server-side, so obvious omissions are caught before
// any network call.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/activate/
func (ss *SSLService) ActivateWithContext(ctx context.Context, args *SSLActivateArgs) (*SSLActivateCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response SSLActivateResponse
	_, err := ss.client.DoXMLWithContext(ctx, args.params(), &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing/invalid field at once, including per-DCV-method
// requirements for the primary domain and each SAN.
func (a *SSLActivateArgs) validate() error {
	missing := make([]string, 0, 4)
	if a.CertificateID < 1 {
		missing = append(missing, "CertificateID")
	}
	if a.CSR == "" {
		missing = append(missing, "CSR")
	}
	if a.AdminEmailAddress == "" {
		missing = append(missing, "AdminEmailAddress")
	}
	if a.DCVMethod != "" {
		missing = append(missing, dcvMissingFields("", a.DCVMethod, a.ApproverEmail)...)
	}
	missing = append(missing, missingSANFields(a.SANs)...)
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map, serializing the
// documented scalar fields and the DCV / multi-SAN blocks.
func (a *SSLActivateArgs) params() map[string]string {
	params := map[string]string{
		"Command":           "namecheap.ssl.activate",
		"CertificateID":     strconv.Itoa(a.CertificateID),
		"CSR":               a.CSR,
		"AdminEmailAddress": a.AdminEmailAddress,
	}
	setIfNotEmpty(params, "WebServerType", a.WebServerType)
	setIfNotEmpty(params, "UniqueValue", a.UniqueValue)
	if a.DCVMethod != "" {
		params["DCVMethod"] = dcvWireValue(a.DCVMethod, a.ApproverEmail)
	}
	applySANParams(params, a.SANs)
	return params
}

// missingSANFields returns every missing field across all SAN entries, prefixed
// with an SANs[i]. label so a caller can fix them in one pass.
func missingSANFields(sans []SANEntry) []string {
	var missing []string
	for i, san := range sans {
		label := fmt.Sprintf("SANs[%d].", i)
		if san.DomainName == "" {
			missing = append(missing, label+"DomainName")
		}
		if san.DCVMethod == "" {
			missing = append(missing, label+"DCVMethod")
		} else {
			missing = append(missing, dcvMissingFields(label, san.DCVMethod, san.ApproverEmail)...)
		}
	}
	return missing
}

// applySANParams serializes each SAN entry as an indexed parameter block
// (SANDomainName[i] / SANDCVMethod[i]), so N host blocks round-trip
// deterministically. The doc does not enumerate these names; they follow the
// documented multi-domain activation contract (flagged in README.md).
func applySANParams(params map[string]string, sans []SANEntry) {
	for i, san := range sans {
		params[fmt.Sprintf("SANDomainName[%d]", i)] = san.DomainName
		params[fmt.Sprintf("SANDCVMethod[%d]", i)] = dcvWireValue(san.DCVMethod, san.ApproverEmail)
	}
}

// sanCommaLists renders a slice of SAN entries as the parallel comma-separated
// DNSNames and DCVMethods lists used by editDCVMethod (doc lines 1088-1089).
func sanCommaLists(sans []SANEntry) (names, methods string) {
	n := make([]string, 0, len(sans))
	m := make([]string, 0, len(sans))
	for _, san := range sans {
		n = append(n, san.DomainName)
		m = append(m, dcvWireValue(san.DCVMethod, san.ApproverEmail))
	}
	return strings.Join(n, ","), strings.Join(m, ",")
}
