package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainsReactivateResponse is the raw envelope for namecheap.domains.reactivate.
type DomainsReactivateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsReactivateCommandResponse `xml:"CommandResponse"`
}

// DomainsReactivateCommandResponse wraps the reactivate result.
type DomainsReactivateCommandResponse struct {
	DomainReactivateResult *DomainsReactivateResult `xml:"DomainReactivateResult"`
}

// DomainsReactivateResult is the outcome of a reactivation. Fields follow the
// reactivate response table in docs/namecheap-api-v2.md (lines 284-290).
type DomainsReactivateResult struct {
	// Domain is the domain that was reactivated (the doc labels this "DomainName").
	Domain *string `xml:"Domain,attr"`
	// IsSuccess indicates whether the domain was reactivated successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
	// ChargedAmount is the total amount charged for the reactivation, kept as an
	// exact decimal string (see Amount).
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
}

// DomainsReactivateArgs are the arguments for ReactivateWithContext. Field names
// and required/optional status follow the reactivate request table in
// docs/namecheap-api-v2.md (lines 275-280).
type DomainsReactivateArgs struct {
	// DomainName is the expired domain to reactivate. Required.
	DomainName string
	// PromotionCode is an optional promotional code.
	PromotionCode string
	// YearsToAdd is the optional number of years to add after expiry. When 0 the
	// parameter is omitted and the API default applies.
	YearsToAdd int
	// IsPremiumDomain must be true to reactivate a premium domain; when true,
	// PremiumPrice is mandatory (see the premium guard on ReactivateWithContext).
	IsPremiumDomain bool
	// PremiumPrice is the agreed reactivation price for a premium domain, as an
	// exact decimal string (see Amount). Required when IsPremiumDomain is true;
	// must be empty otherwise.
	PremiumPrice Amount
}

// ReactivateWithContext reactivates an expired domain.
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
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/reactivate/
func (ds *DomainsService) ReactivateWithContext(ctx context.Context, args *DomainsReactivateArgs) (*DomainsReactivateCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response DomainsReactivateResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := ds.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports a missing domain and applies the premium guard.
func (a *DomainsReactivateArgs) validate() error {
	if a.DomainName == "" {
		return &InvalidArgumentsError{Fields: []string{"DomainName"}}
	}
	return premiumGuard(a.IsPremiumDomain, a.PremiumPrice, "")
}

// params flattens the validated args into the request map.
func (a *DomainsReactivateArgs) params() map[string]string {
	params := map[string]string{
		"Command":    "namecheap.domains.reactivate",
		"DomainName": a.DomainName,
	}
	setIfNotEmpty(params, "PromotionCode", a.PromotionCode)
	if a.YearsToAdd > 0 {
		params["YearsToAdd"] = strconv.Itoa(a.YearsToAdd)
	}
	if a.IsPremiumDomain {
		params["IsPremiumDomain"] = "true"
		params["PremiumPrice"] = a.PremiumPrice.String()
	}
	return params
}
