package namecheap

import (
	"context"
	"encoding/xml"
)

// SSLParseCSRResponse is the raw envelope for namecheap.ssl.parseCSR.
type SSLParseCSRResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLParseCSRCommandResponse `xml:"CommandResponse"`
}

// SSLParseCSRCommandResponse wraps the parseCSR result.
type SSLParseCSRCommandResponse struct {
	SSLParseCSRResult *SSLParseCSRResult `xml:"SSLParseCSRResult"`
}

// SSLParseCSRResult holds the fields decoded from a CSR. They follow the parseCSR
// response table in docs/namecheap-api-v2.md (lines 861-868).
//
// Documented gap. The doc names each field's meaning ("Common name", "Domain
// name", ...) but not the exact XML element names of the response; the tags below
// use the conventional CSR distinguished-name element names nested under
// <CSRDetails>. If a live response uses different element names, only the mapping
// here needs adjusting — the field set matches the doc.
type SSLParseCSRResult struct {
	// CommonName is the hostname the SSL is applied to.
	CommonName *string `xml:"CSRDetails>CommonName"`
	// DomainName is the domain the SSL is applied to.
	DomainName *string `xml:"CSRDetails>DomainName"`
	// Country is the country of the applicant.
	Country *string `xml:"CSRDetails>Country"`
	// OrganisationUnit is the organisation unit of the applicant.
	OrganisationUnit *string `xml:"CSRDetails>OrganisationUnit"`
	// Organisation is the organisation of the applicant.
	Organisation *string `xml:"CSRDetails>Organisation"`
	// State is the state / province information.
	State *string `xml:"CSRDetails>State"`
	// Locality is the locality information.
	Locality *string `xml:"CSRDetails>Locality"`
	// Email is the email address of the applicant.
	Email *string `xml:"CSRDetails>Email"`
}

// ParseCSRWithContext parses a Certificate Signing Request and returns its
// decoded fields. It only transports the CSR string; no key material is read or
// stored. It is an idempotent read and retries on transient failures.
//
// certificateType is optional (doc line 855); pass "" to omit it.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/parse-csr/
func (ss *SSLService) ParseCSRWithContext(ctx context.Context, csr, certificateType string) (*SSLParseCSRCommandResponse, error) {
	if csr == "" {
		return nil, &InvalidArgumentsError{Fields: []string{"csr"}}
	}

	params := map[string]string{
		"Command": "namecheap.ssl.parseCSR",
		"csr":     csr,
	}
	setIfNotEmpty(params, "CertificateType", certificateType)

	var response SSLParseCSRResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
