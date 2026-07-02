package namecheap

import (
	"context"
	"encoding/xml"
)

// SSLGetApproverEmailListResponse is the raw envelope for
// namecheap.ssl.getApproverEmailList.
type SSLGetApproverEmailListResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLGetApproverEmailListCommandResponse `xml:"CommandResponse"`
}

// SSLGetApproverEmailListCommandResponse wraps the getApproverEmailList result.
// Fields follow the response table in docs/namecheap-api-v2.md (lines 887-891).
type SSLGetApproverEmailListCommandResponse struct {
	// DomainEmails are the domain WHOIS email addresses.
	DomainEmails []string `xml:"GetApproverEmailListResult>Domainemails>email"`
	// GenericEmails are the generic email addresses for the domain.
	GenericEmails []string `xml:"GetApproverEmailListResult>Genericemails>email"`
	// ManualEmails are additional approver emails from the provider.
	ManualEmails []string `xml:"GetApproverEmailListResult>Manualemails>email"`
}

// GetApproverEmailListWithContext returns the approver email list for the given
// domain and certificate type. It is an idempotent read and retries on transient
// failures.
//
// Both domainName and certificateType are required (doc lines 882-883).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/get-approver-email-list/
func (ss *SSLService) GetApproverEmailListWithContext(ctx context.Context, domainName, certificateType string) (*SSLGetApproverEmailListCommandResponse, error) {
	missing := make([]string, 0, 2)
	if domainName == "" {
		missing = append(missing, "DomainName")
	}
	if certificateType == "" {
		missing = append(missing, "CertificateType")
	}
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{
		"Command":         "namecheap.ssl.getApproverEmailList",
		"DomainName":      domainName,
		"CertificateType": certificateType,
	}

	var response SSLGetApproverEmailListResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
