package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// UsersAddressSetDefaultResponse is the raw envelope for
// namecheap.users.address.setDefault.
type UsersAddressSetDefaultResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersAddressSetDefaultCommandResponse `xml:"CommandResponse"`
}

// UsersAddressSetDefaultCommandResponse wraps the setDefault result.
type UsersAddressSetDefaultCommandResponse struct {
	AddressSetDefaultResult *UsersAddressSetDefaultResult `xml:"AddressSetDefaultResult"`
}

// UsersAddressSetDefaultResult is the outcome of a setDefault. Field names follow
// the captured sandbox response in
// namecheaptest/fixtures/users_address_setDefault.xml.
type UsersAddressSetDefaultResult struct {
	// Success reports whether the default address was set.
	Success *bool `xml:"Success,attr"`
	// AddressID is the id of the new default address profile. The response
	// attribute is AddressId.
	AddressID *int `xml:"AddressId,attr"`
}

// SetDefaultWithContext marks an address-book entry as the account default.
//
// It is a non-idempotent write but not charge-bearing; it uses the standard
// idempotent transport. addressID must not be negative: 0 makes the account's
// primary address the default again.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/set-default/
func (uas *UsersAddressService) SetDefaultWithContext(ctx context.Context, addressID int) (*UsersAddressSetDefaultCommandResponse, error) {
	if addressID < 0 {
		return nil, &InvalidArgumentsError{Fields: []string{"AddressId"}}
	}

	params := map[string]string{
		"Command":   "namecheap.users.address.setDefault",
		"AddressId": strconv.Itoa(addressID),
	}

	var response UsersAddressSetDefaultResponse
	_, err := uas.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
