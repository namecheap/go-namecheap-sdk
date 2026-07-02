package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLResendApproverEmailResponse is the raw envelope for
// namecheap.ssl.resendApproverEmail.
type SSLResendApproverEmailResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLResendApproverEmailCommandResponse `xml:"CommandResponse"`
}

// SSLResendApproverEmailCommandResponse wraps the resendApproverEmail result.
type SSLResendApproverEmailCommandResponse struct {
	SSLResendApproverEmailResult *SSLResendApproverEmailResult `xml:"SSLResendApproverEmailResult"`
}

// SSLResendApproverEmailResult is the outcome of a resend request. Fields follow
// the response table in docs/namecheap-api-v2.md (lines 929-930).
type SSLResendApproverEmailResult struct {
	// ID is the unique integer identifying the certificate.
	ID *int `xml:"ID,attr"`
	// IsSuccess reports whether the approver email was resent successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// ResendApproverEmailWithContext resends the approver email for a certificate and
// also serves as the retry mechanism for HTTP/DNS validation (doc lines 913-915).
// It is idempotent (safe to resend) and retries on transient failures.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/resend-approver-email/
func (ss *SSLService) ResendApproverEmailWithContext(ctx context.Context, certificateID int) (*SSLResendApproverEmailCommandResponse, error) {
	if certificateID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"CertificateID"}, Reason: "CertificateID must be a positive integer"}
	}

	params := map[string]string{
		"Command":       "namecheap.ssl.resendApproverEmail",
		"CertificateID": strconv.Itoa(certificateID),
	}

	var response SSLResendApproverEmailResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
