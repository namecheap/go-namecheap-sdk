package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLResendFulfillmentEmailResponse is the raw envelope for
// namecheap.ssl.resendfulfillmentemail.
type SSLResendFulfillmentEmailResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLResendFulfillmentEmailCommandResponse `xml:"CommandResponse"`
}

// SSLResendFulfillmentEmailCommandResponse wraps the resendfulfillmentemail
// result.
type SSLResendFulfillmentEmailCommandResponse struct {
	SSLResendFulfillmentEmailResult *SSLResendFulfillmentEmailResult `xml:"SSLResendFulfillmentEmailResult"`
}

// SSLResendFulfillmentEmailResult is the outcome of a resend. Fields follow the
// resendfulfillmentemail response table in docs/namecheap-api-v2.md
// (lines 1022-1023).
type SSLResendFulfillmentEmailResult struct {
	// ID is the unique integer identifying the certificate.
	ID *int `xml:"ID,attr"`
	// IsSuccess reports whether the fulfillment email was resent successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// ResendFulfillmentEmailWithContext resends the fulfilment email that carries the
// issued certificate. Resending an email is safe, so the call is treated as
// idempotent and retries on transient failures.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/resend-fulfillment-email/
func (ss *SSLService) ResendFulfillmentEmailWithContext(ctx context.Context, certificateID int) (*SSLResendFulfillmentEmailCommandResponse, error) {
	if certificateID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"CertificateID"}, Reason: "CertificateID must be a positive integer"}
	}

	params := map[string]string{
		"Command":       "namecheap.ssl.resendfulfillmentemail",
		"CertificateID": strconv.Itoa(certificateID),
	}

	var response SSLResendFulfillmentEmailResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
