package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLEditDCVMethodResponse is the raw envelope for namecheap.ssl.editDCVMethod.
type SSLEditDCVMethodResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLEditDCVMethodCommandResponse `xml:"CommandResponse"`
}

// SSLEditDCVMethodCommandResponse wraps the editDCVMethod result.
type SSLEditDCVMethodCommandResponse struct {
	SSLEditDCVMethodResult *SSLEditDCVMethodResult `xml:"SSLEditDCVMethodResult"`
}

// SSLEditDCVMethodResult is the outcome of a DCV-method edit. Fields follow the
// response table in docs/namecheap-api-v2.md (lines 1093-1099).
type SSLEditDCVMethodResult struct {
	// ID is the unique integer identifying the certificate.
	ID *int `xml:"ID,attr"`
	// IsSuccess reports whether the certificate DCV was updated.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// HTTPDCValidation carries the HTTP DCV file details when HTTP validation is set.
	HTTPDCValidation *SSLHTTPDCValidation `xml:"HttpDCValidation"`
}

// SSLHTTPDCValidation carries the HTTP domain-control-validation file details.
type SSLHTTPDCValidation struct {
	// ValueAvailable reports whether an HTTP_CSR_HASH was set for at least one domain.
	ValueAvailable *bool `xml:"ValueAvailable,attr"`
	// FileName is the file name for the HTTP DCV text file.
	FileName *string `xml:"FileName"`
	// FileContent is the content for the HTTP DCV text file.
	FileContent *string `xml:"FileContent"`
}

// SSLEditDCVMethodArgs are the arguments for EditDCVMethodWithContext. Field names
// follow the editDCVMethod request table in docs/namecheap-api-v2.md
// (lines 1084-1089).
//
// Use either the single-domain form (set DCVMethod, plus ApproverEmail for email
// validation) or the multi-domain form (set SANs, serialized to the comma-
// separated DNSNames / DCVMethods lists). At least one of the two must be present.
type SSLEditDCVMethodArgs struct {
	// CertificateID is the certificate to update. Required.
	CertificateID int
	// DCVMethod is the single-domain DCV method (doc line 1087). Set this OR SANs.
	DCVMethod DCVMethod
	// ApproverEmail is the approver email for single-domain email validation;
	// required only when DCVMethod is DCVMethodEmail.
	ApproverEmail string
	// SANs is the multi-domain form: each domain plus its DCV method, serialized as
	// the parallel DNSNames / DCVMethods comma lists (doc lines 1088-1089).
	SANs []SANEntry
}

// EditDCVMethodWithContext sets a new domain-control-validation method for a
// certificate (or retries validation). It is idempotent and retries on transient
// failures. Arguments are validated client-side first, listing every missing field
// at once.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/edit-dcv-method/
func (ss *SSLService) EditDCVMethodWithContext(ctx context.Context, args *SSLEditDCVMethodArgs) (*SSLEditDCVMethodCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response SSLEditDCVMethodResponse
	_, err := ss.client.DoXMLWithContext(ctx, args.params(), &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing/invalid field at once. It requires a positive
// CertificateID and exactly one populated form (single DCVMethod or SANs), then
// applies per-method requirements.
func (a *SSLEditDCVMethodArgs) validate() error {
	missing := make([]string, 0, 2)
	if a.CertificateID < 1 {
		missing = append(missing, "CertificateID")
	}
	switch {
	case len(a.SANs) > 0:
		missing = append(missing, missingSANFields(a.SANs)...)
	case a.DCVMethod != "":
		missing = append(missing, dcvMissingFields("", a.DCVMethod, a.ApproverEmail)...)
	default:
		missing = append(missing, "DCVMethod")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map, choosing the single-
// domain or multi-domain serialization.
func (a *SSLEditDCVMethodArgs) params() map[string]string {
	params := map[string]string{
		"Command":       "namecheap.ssl.editDCVMethod",
		"CertificateID": strconv.Itoa(a.CertificateID),
	}
	if len(a.SANs) > 0 {
		names, methods := sanCommaLists(a.SANs)
		params["DNSNames"] = names
		params["DCVMethods"] = methods
	} else {
		params["DCVMethod"] = dcvWireValue(a.DCVMethod, a.ApproverEmail)
	}
	return params
}
