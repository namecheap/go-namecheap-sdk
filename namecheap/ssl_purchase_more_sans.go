package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// SSLPurchaseMoreSansResponse is the raw envelope for
// namecheap.ssl.purchasemoresans.
type SSLPurchaseMoreSansResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *SSLPurchaseMoreSansCommandResponse `xml:"CommandResponse"`
}

// SSLPurchaseMoreSansCommandResponse wraps the purchasemoresans result.
type SSLPurchaseMoreSansCommandResponse struct {
	SSLPurchaseMoreSansResult *SSLPurchaseMoreSansResult `xml:"SSLPurchaseMoreSansResult"`
}

// SSLPurchaseMoreSansResult is the outcome of purchasing additional SANs. Fields
// follow the purchasemoresans response table in docs/namecheap-api-v2.md
// (lines 1044-1050).
type SSLPurchaseMoreSansResult struct {
	// IsSuccess reports whether more SANs were purchased.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
	// ChargedAmount is the amount charged, kept as an exact decimal string (see Amount).
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
	// CertificateID is the unique integer identifying the SSL certificate.
	CertificateID *int `xml:"CertificateID,attr"`
	// SSLType is the type of SSL certificate.
	SSLType *string `xml:"SSLType,attr"`
	// SANSCount is the number of add-on domains after the purchase.
	SANSCount *int `xml:"SANSCount,attr"`
}

// SSLPurchaseMoreSansArgs are the arguments for PurchaseMoreSansWithContext.
// Field names and required status follow the purchasemoresans request table in
// docs/namecheap-api-v2.md (lines 1036-1038).
type SSLPurchaseMoreSansArgs struct {
	// CertificateID is the certificate to add SANs to. Required.
	CertificateID int
	// NumberOfSANSToAdd is the number of add-on domains to order. Required (>= 1).
	NumberOfSANSToAdd int
}

// PurchaseMoreSansWithContext purchases additional add-on domains (SANs) for an
// already-purchased certificate.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could double-charge). Only
// the pre-execution HTTP 405 rate-limit signal is retried. Reconcile ambiguous
// failures via GetInfoWithContext / the account order history.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/ssl/purchase-more-sans/
func (ss *SSLService) PurchaseMoreSansWithContext(ctx context.Context, args *SSLPurchaseMoreSansArgs) (*SSLPurchaseMoreSansCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response SSLPurchaseMoreSansResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := ss.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing required field at once.
func (a *SSLPurchaseMoreSansArgs) validate() error {
	missing := make([]string, 0, 2)
	if a.CertificateID < 1 {
		missing = append(missing, "CertificateID")
	}
	if a.NumberOfSANSToAdd < 1 {
		missing = append(missing, "NumberOfSANSToAdd")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map.
func (a *SSLPurchaseMoreSansArgs) params() map[string]string {
	return map[string]string{
		"Command":           "namecheap.ssl.purchasemoresans",
		"CertificateID":     strconv.Itoa(a.CertificateID),
		"NumberOfSANSToAdd": strconv.Itoa(a.NumberOfSANSToAdd),
	}
}
