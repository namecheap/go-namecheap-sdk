package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainsRenewResponse is the raw envelope for namecheap.domains.renew.
type DomainsRenewResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsRenewCommandResponse `xml:"CommandResponse"`
}

// DomainsRenewCommandResponse wraps the renew result.
type DomainsRenewCommandResponse struct {
	DomainRenewResult *DomainsRenewResult `xml:"DomainRenewResult"`
}

// DomainsRenewResult is the outcome of a renewal. Fields follow the renew
// response table in docs/namecheap-api-v2.md (lines 311-319).
type DomainsRenewResult struct {
	// DomainName is the domain that was renewed.
	DomainName *string `xml:"DomainName,attr"`
	// DomainID is the unique integer identifying the domain.
	DomainID *int `xml:"DomainID,attr"`
	// Renew indicates whether the domain was renewed successfully.
	Renew *bool `xml:"Renew,attr"`
	// ChargedAmount is the total amount charged for the renewal, kept as an exact
	// decimal string (see Amount).
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
}

// DomainsRenewArgs are the arguments for RenewWithContext. Field names and
// required/optional status follow the renew request table in
// docs/namecheap-api-v2.md (lines 302-308).
type DomainsRenewArgs struct {
	// DomainName is the domain to renew. Required.
	DomainName string
	// Years is the number of years to renew for. Required (>= 1).
	Years int
	// PromotionCode is an optional promotional code.
	PromotionCode string
	// IsPremiumDomain must be true to renew a premium domain; when true,
	// PremiumPrice is mandatory (see the premium guard on RenewWithContext).
	IsPremiumDomain bool
	// PremiumPrice is the agreed renewal price for a premium domain, as an exact
	// decimal string (see Amount). Required when IsPremiumDomain is true; must be
	// empty otherwise.
	PremiumPrice Amount
}

// RenewWithContext renews an expiring domain.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could double-charge), so
// the caller must reconcile such failures via the account order history.
//
// The same premium guard as CreateWithContext applies: when IsPremiumDomain is
// true PremiumPrice must be set, and when it is false PremiumPrice must be
// empty. A violation returns an *InvalidArgumentsError before the request is
// sent, so no charge can occur.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/renew/
func (ds *DomainsService) RenewWithContext(ctx context.Context, args *DomainsRenewArgs) (*DomainsRenewCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response DomainsRenewResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := ds.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports missing required fields and applies the premium guard.
func (a *DomainsRenewArgs) validate() error {
	missing := make([]string, 0, 2)
	if a.DomainName == "" {
		missing = append(missing, "DomainName")
	}
	if a.Years < 1 {
		missing = append(missing, "Years")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return premiumGuard(a.IsPremiumDomain, a.PremiumPrice, "")
}

// params flattens the validated args into the request map.
func (a *DomainsRenewArgs) params() map[string]string {
	params := map[string]string{
		"Command":    "namecheap.domains.renew",
		"DomainName": a.DomainName,
		"Years":      strconv.Itoa(a.Years),
	}
	setIfNotEmpty(params, "PromotionCode", a.PromotionCode)
	if a.IsPremiumDomain {
		params["IsPremiumDomain"] = "true"
		params["PremiumPrice"] = a.PremiumPrice.String()
	}
	return params
}
