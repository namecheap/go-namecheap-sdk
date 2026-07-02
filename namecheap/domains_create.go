package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
)

// DomainsCreateResponse is the raw envelope for namecheap.domains.create.
type DomainsCreateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsCreateCommandResponse `xml:"CommandResponse"`
}

// DomainsCreateCommandResponse wraps the create result.
type DomainsCreateCommandResponse struct {
	DomainCreateResult *DomainsCreateResult `xml:"DomainCreateResult"`
}

// DomainsCreateResult is the outcome of a registration. Fields follow the
// create response table in docs/namecheap-api-v2.md (lines 167-174).
type DomainsCreateResult struct {
	// Domain is the domain name that was registered.
	Domain *string `xml:"Domain,attr"`
	// Registered indicates whether the domain was registered successfully.
	Registered *bool `xml:"Registered,attr"`
	// ChargedAmount is the total amount charged for the registration, kept as an
	// exact decimal string (see Amount).
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
	// DomainID is the unique integer identifying the domain.
	DomainID *int `xml:"DomainID,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
}

// DomainsCreateArgs are the arguments for CreateWithContext. Field names and
// required/optional status follow the create request table in
// docs/namecheap-api-v2.md (lines 138-163).
type DomainsCreateArgs struct {
	// DomainName is the domain to register. Required.
	DomainName string
	// Years is the number of years to register for. Required; the API default
	// is 2 but this SDK requires an explicit value >= 1 for a charge-bearing call.
	Years int
	// PromotionCode is an optional promotional (coupon) code.
	PromotionCode string

	// Registrant, Tech, Admin and AuxBilling are the four required contact
	// blocks. Every required ContactInfo field must be set on each.
	Registrant ContactInfo
	Tech       ContactInfo
	Admin      ContactInfo
	AuxBilling ContactInfo

	// Nameservers is an optional list of custom nameservers; it is sent
	// comma-joined. When empty the domain uses Namecheap's default DNS.
	Nameservers []string
	// AddFreeWhoisguard, when non-nil, adds (true) or omits (false) free domain
	// privacy. Nil leaves the API default (Yes).
	AddFreeWhoisguard *bool
	// WGEnabled, when non-nil, enables (true) or disables (false) domain privacy.
	// Nil leaves the API default (No).
	WGEnabled *bool

	// IsPremiumDomain must be true to register a premium domain. When true,
	// PremiumPrice is mandatory (see the premium guard on CreateWithContext).
	IsPremiumDomain bool
	// PremiumPrice is the agreed registration price for a premium domain, as an
	// exact decimal string (see Amount). Required when IsPremiumDomain is true;
	// must be empty otherwise.
	PremiumPrice Amount
	// EapFee is the Early Access Program fee for a premium domain, as an exact
	// decimal string (see Amount). Only meaningful when IsPremiumDomain is true.
	EapFee Amount
}

// CreateWithContext registers a new domain name.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could double-charge), so
// the caller must reconcile such failures via the account order history.
//
// Premium guard (money safety). Before any network call, arguments are
// validated and the premium fields are checked so an accidental premium purchase
// is impossible without explicit acknowledgment:
//   - if IsPremiumDomain is true, PremiumPrice must be a non-empty amount;
//   - if IsPremiumDomain is false, neither PremiumPrice nor EapFee may be set.
//
// A violation returns an *InvalidArgumentsError before the request is sent, so
// no charge can occur. Missing required contact fields are reported the same
// way, all at once.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/create/
func (ds *DomainsService) CreateWithContext(ctx context.Context, args *DomainsCreateArgs) (*DomainsCreateCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response DomainsCreateResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := ds.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports all missing required fields at once and then applies the
// premium guard.
func (a *DomainsCreateArgs) validate() error {
	missing := make([]string, 0, 4)
	if a.DomainName == "" {
		missing = append(missing, "DomainName")
	}
	if a.Years < 1 {
		missing = append(missing, "Years")
	}
	missing = append(missing, missingContactFields(a.Registrant, a.Tech, a.Admin, a.AuxBilling)...)
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return premiumGuard(a.IsPremiumDomain, a.PremiumPrice, a.EapFee)
}

// params flattens the validated args into the request map.
func (a *DomainsCreateArgs) params() map[string]string {
	params := map[string]string{
		"Command":    "namecheap.domains.create",
		"DomainName": a.DomainName,
		"Years":      strconv.Itoa(a.Years),
	}
	setIfNotEmpty(params, "PromotionCode", a.PromotionCode)
	applyContacts(params, a.Registrant, a.Tech, a.Admin, a.AuxBilling)
	if len(a.Nameservers) > 0 {
		params["Nameservers"] = strings.Join(a.Nameservers, ",")
	}
	if a.AddFreeWhoisguard != nil {
		params["AddFreeWhoisguard"] = yesNo(*a.AddFreeWhoisguard)
	}
	if a.WGEnabled != nil {
		params["WGEnabled"] = yesNo(*a.WGEnabled)
	}
	if a.IsPremiumDomain {
		params["IsPremiumDomain"] = "true"
		params["PremiumPrice"] = a.PremiumPrice.String()
		setIfNotEmpty(params, "EapFee", a.EapFee.String())
	}
	return params
}

// premiumGuard enforces the money-safety contract shared by every
// charge-bearing call that accepts premium pricing. See CreateWithContext for
// the documented contract.
func premiumGuard(isPremium bool, premiumPrice, eapFee Amount) error {
	if isPremium {
		if premiumPrice == "" {
			return &InvalidArgumentsError{
				Fields: []string{"PremiumPrice"},
				Reason: "IsPremiumDomain is set but PremiumPrice is empty; a premium purchase requires an explicit price",
			}
		}
		return nil
	}

	stray := make([]string, 0, 2)
	if premiumPrice != "" {
		stray = append(stray, "PremiumPrice")
	}
	if eapFee != "" {
		stray = append(stray, "EapFee")
	}
	if len(stray) > 0 {
		return &InvalidArgumentsError{
			Fields: stray,
			Reason: "premium pricing is set while IsPremiumDomain is false",
		}
	}
	return nil
}

// yesNo maps a boolean onto Namecheap's "Yes"/"No" string convention.
func yesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}
