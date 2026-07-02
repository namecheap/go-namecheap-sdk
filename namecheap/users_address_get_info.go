package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// UsersAddressGetInfoResponse is the raw envelope for
// namecheap.users.address.getInfo.
type UsersAddressGetInfoResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersAddressGetInfoCommandResponse `xml:"CommandResponse"`
}

// UsersAddressGetInfoCommandResponse wraps the getInfo result.
type UsersAddressGetInfoCommandResponse struct {
	GetAddressInfoResult *UsersAddressGetInfoResult `xml:"GetAddressInfoResult"`
}

// UsersAddressGetInfoResult is the full stored address. The doc gives no response
// table for address.getInfo (docs/namecheap-api-v2.md lines 1391-1408, only error
// codes), so the fields mirror the create request fields (lines 1347-1365), which
// getInfo echoes back; on the wire they are child elements. Every field is a
// pointer so a field the server omits stays nil rather than an empty value.
type UsersAddressGetInfoResult struct {
	// AddressID is the unique integer identifying the address profile.
	AddressID *int `xml:"AddressId"`
	// AddressName is the address label.
	AddressName *string `xml:"AddressName"`
	// DefaultYN reports whether this is the default address.
	DefaultYN *bool `xml:"DefaultYN"`
	// EmailAddress is the contact e-mail address.
	EmailAddress *string `xml:"EmailAddress"`
	// FirstName is the contact's first name.
	FirstName *string `xml:"FirstName"`
	// LastName is the contact's last name.
	LastName *string `xml:"LastName"`
	// JobTitle is the contact's job designation.
	JobTitle *string `xml:"JobTitle"`
	// Organization is the contact's organization.
	Organization *string `xml:"Organization"`
	// Address1 is the primary street address.
	Address1 *string `xml:"Address1"`
	// Address2 is the secondary street address.
	Address2 *string `xml:"Address2"`
	// City is the contact's city.
	City *string `xml:"City"`
	// StateProvince is the state or province.
	StateProvince *string `xml:"StateProvince"`
	// StateProvinceChoice is the state/province choice.
	StateProvinceChoice *string `xml:"StateProvinceChoice"`
	// Zip is the postal/ZIP code.
	Zip *string `xml:"Zip"`
	// Country is the two-letter country code.
	Country *string `xml:"Country"`
	// Phone is the phone number in +NNN.NNNNNNNNNN format.
	Phone *string `xml:"Phone"`
	// PhoneExt is the phone extension.
	PhoneExt *string `xml:"PhoneExt"`
	// Fax is the fax number.
	Fax *string `xml:"Fax"`
}

// GetInfoWithContext returns the full stored address for the given id.
//
// It is a read-only, idempotent call. addressID must be positive.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/get-info/
func (uas *UsersAddressService) GetInfoWithContext(ctx context.Context, addressID int) (*UsersAddressGetInfoCommandResponse, error) {
	if addressID <= 0 {
		return nil, &InvalidArgumentsError{Fields: []string{"AddressId"}}
	}

	params := map[string]string{
		"Command":   "namecheap.users.address.getInfo",
		"AddressId": strconv.Itoa(addressID),
	}

	var response UsersAddressGetInfoResponse
	_, err := uas.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// ToContactInfo converts a fetched address into a domain ContactInfo so the
// stored entry can feed the contact blocks of domains.create. It applies the same
// field mapping as UsersAddressDetails.ToContactInfo (Zip -> PostalCode,
// Organization -> OrganizationName) and treats a nil field as an empty string.
func (r *UsersAddressGetInfoResult) ToContactInfo() ContactInfo {
	if r == nil {
		return ContactInfo{}
	}
	return ContactInfo{
		FirstName:        derefString(r.FirstName),
		LastName:         derefString(r.LastName),
		Address1:         derefString(r.Address1),
		City:             derefString(r.City),
		StateProvince:    derefString(r.StateProvince),
		PostalCode:       derefString(r.Zip),
		Country:          derefString(r.Country),
		Phone:            derefString(r.Phone),
		EmailAddress:     derefString(r.EmailAddress),
		OrganizationName: derefString(r.Organization),
		JobTitle:         derefString(r.JobTitle),
		Address2:         derefString(r.Address2),
	}
}

// derefString returns *p, or "" when p is nil.
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
