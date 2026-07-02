package namecheap

import (
	"context"
	"encoding/xml"
	"strings"
)

// UsersGetPricingResponse is the raw envelope for namecheap.users.getPricing.
type UsersGetPricingResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersGetPricingCommandResponse `xml:"CommandResponse"`
}

// UsersGetPricingCommandResponse wraps the getPricing result.
type UsersGetPricingCommandResponse struct {
	UserGetPricingResult *UsersGetPricingResult `xml:"UserGetPricingResult"`
}

// UsersGetPricingResult is the root of the deeply nested price sheet. It holds
// one entry per requested product type (DOMAIN, SSLCERTIFICATE, WHOISGUARD).
//
// The response is large and slow-changing: it is safe (and recommended) to fetch
// it once and cache it rather than call getPricing on a hot path. Consumers own
// the caching policy (see the "Out of scope" note in issue #117).
type UsersGetPricingResult struct {
	// ProductTypes lists the pricing tree for each product type in the response,
	// e.g. "domains". Element/attribute names follow the getPricing response table
	// in docs/namecheap-api-v2.md (lines 1123-1132).
	ProductTypes []PricingProductType `xml:"ProductType"`
}

// PricingProductType is one product type node (the doc's "ProductType Name",
// line 1125) and its categories.
type PricingProductType struct {
	// Name is the product-type identifier, e.g. "domains".
	Name string `xml:"Name,attr"`
	// ProductCategories are the actions/categories under this type, e.g.
	// "register", "renew", "transfer".
	ProductCategories []PricingProductCategory `xml:"ProductCategory"`
}

// PricingProductCategory is one category node (the doc's "ProductCategory Name",
// line 1126), typically corresponding to an action such as register/renew.
type PricingProductCategory struct {
	// Name is the category/action identifier, e.g. "register".
	Name string `xml:"Name,attr"`
	// Products are the individual products under this category, e.g. one per TLD.
	Products []PricingProduct `xml:"Product"`
}

// PricingProduct is one product node (the doc's "Product Name", line 1127) and
// its per-duration price tiers.
type PricingProduct struct {
	// Name is the product identifier, e.g. the TLD "com".
	Name string `xml:"Name,attr"`
	// Prices are the price tiers for this product, one per duration.
	Prices []Price `xml:"Price"`
}

// Price is a single price tier. Field names and money semantics follow the
// getPricing response table in docs/namecheap-api-v2.md (lines 1128-1132). Every
// monetary field is an Amount (an exact decimal string) so a charge-bearing price
// is never silently mangled by binary floating point.
type Price struct {
	// Duration is the term length, e.g. 1 (see DurationType for the unit).
	Duration int `xml:"Duration,attr"`
	// DurationType is the unit of Duration, e.g. "YEAR".
	DurationType string `xml:"DurationType,attr"`
	// Price is the server-resolved final price. Per the doc (line 1130) it is
	// derived from "regular, userprice, special, promo, or tier price", so any
	// active promotion is already reflected here. See EffectivePrice.
	Price Amount `xml:"Price,attr"`
	// RegularPrice is the public list price (doc line 1131).
	RegularPrice Amount `xml:"RegularPrice,attr"`
	// YourPrice is the account/user-specific price (doc line 1132).
	YourPrice Amount `xml:"YourPrice,attr"`
}

// UsersGetPricingArgs are the arguments for GetPricingWithContext. Field names
// and required/optional status follow the getPricing request table in
// docs/namecheap-api-v2.md (lines 1113-1119).
type UsersGetPricingArgs struct {
	// ProductType selects the price sheet to return. Required. Canonical values
	// are "DOMAIN", "SSLCERTIFICATE" and "WHOISGUARD".
	ProductType *string
	// ProductCategory optionally narrows the result to a single category.
	ProductCategory *string
	// ActionName optionally narrows the result to a single action, e.g.
	// "REGISTER", "RENEW" or "TRANSFER".
	ActionName *string
	// ProductName optionally narrows the result to a single product, e.g. "com".
	ProductName *string
	// PromotionCode is an optional promotional (coupon) code applied to the sheet.
	PromotionCode *string
}

// GetPricingWithContext returns Namecheap's price sheet for a product type.
//
// It is a read-only, idempotent call. The response is deeply nested
// (ProductType -> ProductCategory -> Product -> Price tiers) and models the doc's
// element/attribute names exactly; use PriceFor for the common single-tier
// lookup. Because the sheet is large and changes rarely, prefer fetching it once
// and caching it over calling this on a hot path.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/get-pricing/
func (us *UsersService) GetPricingWithContext(ctx context.Context, args *UsersGetPricingArgs) (*UsersGetPricingCommandResponse, error) {
	if args == nil || args.ProductType == nil || *args.ProductType == "" {
		return nil, &InvalidArgumentsError{Fields: []string{"ProductType"}}
	}

	var response UsersGetPricingResponse
	_, err := us.client.DoXMLWithContext(ctx, args.params(), &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// params flattens the validated args into the request map.
func (a *UsersGetPricingArgs) params() map[string]string {
	params := map[string]string{
		"Command":     "namecheap.users.getPricing",
		"ProductType": *a.ProductType,
	}
	setIfNotNil(params, "ProductCategory", a.ProductCategory)
	setIfNotNil(params, "ActionName", a.ActionName)
	setIfNotNil(params, "ProductName", a.ProductName)
	setIfNotNil(params, "PromotionCode", a.PromotionCode)
	return params
}

// PriceFor resolves the price tier for an action (a product category such as
// "REGISTER"), a product name (such as "com") and a term length in years. It is
// the flattening helper for the 90% lookup: it walks every product type, matches
// the category by action, the product by name and the tier by an annual duration
// equal to years. Matching is case-insensitive so "REGISTER"/"register" and
// "COM"/"com" both resolve. It returns the matching Price and true, or the zero
// Price and false when nothing matches.
func (r *UsersGetPricingResult) PriceFor(action, productName string, years int) (Price, bool) {
	if r == nil {
		return Price{}, false
	}
	for i := range r.ProductTypes {
		categories := r.ProductTypes[i].ProductCategories
		for j := range categories {
			if !strings.EqualFold(categories[j].Name, action) {
				continue
			}
			if p, ok := categories[j].priceFor(productName, years); ok {
				return p, true
			}
		}
	}
	return Price{}, false
}

// priceFor resolves the tier for productName/years within a single category.
func (c *PricingProductCategory) priceFor(productName string, years int) (Price, bool) {
	for i := range c.Products {
		if !strings.EqualFold(c.Products[i].Name, productName) {
			continue
		}
		return c.Products[i].priceFor(years)
	}
	return Price{}, false
}

// priceFor resolves the annual tier for years within a single product.
func (p *PricingProduct) priceFor(years int) (Price, bool) {
	for i := range p.Prices {
		if p.Prices[i].Duration == years && strings.EqualFold(p.Prices[i].DurationType, "YEAR") {
			return p.Prices[i], true
		}
	}
	return Price{}, false
}

// EffectivePrice returns the amount a caller would actually pay for this tier,
// applying the documented precedence over the tier's price attributes
// (docs/namecheap-api-v2.md lines 1130-1132):
//
//  1. Price        — the server-resolved final price, which per the doc (line
//  1130. already reflects "regular, userprice, special, promo, or tier
//     price"; this is where an active promotion surfaces;
//  2. YourPrice    — the account/user-specific price (line 1132);
//  3. RegularPrice — the public list price (line 1131).
//
// The first attribute that is present and represents a positive amount wins; a
// missing or zero-valued attribute falls through to the next. When none is
// positive it returns RegularPrice unchanged (possibly empty), so the caller
// still sees the raw server value rather than a fabricated one.
func (p Price) EffectivePrice() Amount {
	for _, a := range []Amount{p.Price, p.YourPrice, p.RegularPrice} {
		if isPositiveAmount(a) {
			return a
		}
	}
	return p.RegularPrice
}

// isPositiveAmount reports whether a is a present, positive money value. It is
// decimal-safe: it never parses the amount to a float (see Amount), it only
// checks for the presence of a non-zero digit, so "0.00" and "" are false while
// "8.88" and "10.00" are true.
func isPositiveAmount(a Amount) bool {
	for _, r := range strings.TrimSpace(string(a)) {
		if r >= '1' && r <= '9' {
			return true
		}
	}
	return false
}

// setIfNotNil writes key=*value into params only when value is non-nil and the
// pointed-to string is non-empty.
func setIfNotNil(params map[string]string, key string, value *string) {
	if value != nil && *value != "" {
		params[key] = *value
	}
}
