package namecheap

import (
	"encoding/xml"
	"fmt"
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
	DomainCheckResult *DomainCheckResult `xml:"DomainCheckResult"`
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

func (ds *DomainsService) Check(domain string) (*DomainsCheckCommandResponse, error) {
	var response DomainsCheckResponse

	params := map[string]string{
		"Command":    "namecheap.domains.check",
		"DomainList": domain,
	}

	_, err := ds.client.DoXML(params, &response)
	if err != nil {
		return nil, err
	}
	if response.Errors != nil && len(*response.Errors) > 0 {
		apiErr := (*response.Errors)[0]

		return nil, fmt.Errorf("%s (%s)", *apiErr.Message, *apiErr.Number)
	}

	return response.CommandResponse, nil
}
