package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLRevokeCertificateResponse is the raw envelope for
// namecheap.ssl.revokecertificate.
type SSLRevokeCertificateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLRevokeCertificateCommandResponse `xml:"CommandResponse"`
}

// SSLRevokeCertificateCommandResponse wraps the revokecertificate result.
type SSLRevokeCertificateCommandResponse struct {
	SSLRevokeCertificateResult *SSLRevokeCertificateResult `xml:"SSLRevokeCertificateResult"`
}

// SSLRevokeCertificateResult is the outcome of a revoke. Fields follow the
// revokecertificate response table in docs/namecheap-api-v2.md (lines 1071-1072).
type SSLRevokeCertificateResult struct {
	// IsSuccess reports whether the certificate was revoked successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// CertificateID is the unique integer identifying the certificate.
	CertificateID *int `xml:"CertificateID,attr"`
}

// RevokeCertificateWithContext revokes a re-issued SSL certificate.
//
// It is not charge-bearing and re-firing it is safe (revoking an already-revoked
// certificate leaves the same end state), so it is treated as idempotent and
// retries on transient failures.
//
// Both certificateID and certificateType are required (doc lines 1064-1065).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/revoke-certificate/
func (ss *SSLService) RevokeCertificateWithContext(ctx context.Context, certificateID int, certificateType string) (*SSLRevokeCertificateCommandResponse, error) {
	missing := make([]string, 0, 2)
	if certificateID < 1 {
		missing = append(missing, "CertificateID")
	}
	if certificateType == "" {
		missing = append(missing, "CertificateType")
	}
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{
		"Command":         "namecheap.ssl.revokecertificate",
		"CertificateID":   strconv.Itoa(certificateID),
		"CertificateType": certificateType,
	}

	var response SSLRevokeCertificateResponse
	_, err := ss.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
