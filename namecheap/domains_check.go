package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
)

type DomainsCheckResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsCheckCommandResponse `xml:"CommandResponse"`
}

type DomainsCheckCommandResponse struct {
	DomainCheckResults *[]DomainCheckResult `xml:"DomainCheckResult"`
}

type DomainCheckResult struct {
	Domain                   *string  `xml:"Domain,attr"`
	IsAvailable              *bool    `xml:"Available,attr"`
	IsPremiumName            *bool    `xml:"IsPremiumName,attr"`
	PremiumRegistrationPrice *float64 `xml:"PremiumRegistrationPrice,attr"`
	PremiumRenewalPrice      *float64 `xml:"PremiumRenewalPrice,attr"`
	PremiumRestorePrice      *float64 `xml:"PremiumRestorePrice,attr"`
	PremiumTransferPrice     *float64 `xml:"PremiumTransferPrice,attr"`
	IcannFee                 *float64 `xml:"IcannFee,attr"`
	EapFee                   *float64 `xml:"EapFee,attr"`
}

// CheckWithContext returns availability and pricing information for one or more domains.
// The Namecheap API accepts up to 50 domains per call.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/check/
func (ds *DomainsService) CheckWithContext(ctx context.Context, domains ...string) (*DomainsCheckCommandResponse, error) {
	var response DomainsCheckResponse

	params := map[string]string{
		"Command":    "namecheap.domains.check",
		"DomainList": strings.Join(domains, ","),
	}

	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	if response.Errors != nil && len(*response.Errors) > 0 {
		apiErr := (*response.Errors)[0]

		return nil, fmt.Errorf("%s (%s)", *apiErr.Message, *apiErr.Number)
	}

	return response.CommandResponse, nil
}

// Check returns availability and pricing information for one or more domains.
//
// Deprecated: Check runs without a context. Use CheckWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (ds *DomainsService) Check(domains ...string) (*DomainsCheckCommandResponse, error) {
	return ds.CheckWithContext(context.Background(), domains...)
}
