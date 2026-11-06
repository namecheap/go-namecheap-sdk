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
// idempotent transport. addressID must be positive, so an unset id cannot change
// the account default; to make the primary address the default again, use
// SetPrimaryDefaultWithContext.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/set-default/
func (uas *UsersAddressService) SetDefaultWithContext(ctx context.Context, addressID int) (*UsersAddressSetDefaultCommandResponse, error) {
	if addressID <= 0 {
		return nil, &InvalidArgumentsError{Fields: []string{"AddressId"}}
	}
	return uas.setDefault(ctx, addressID)
}

// SetPrimaryDefaultWithContext makes the account's primary address
// (PrimaryAddressID) the default again. The API accepts AddressId=0 for it
// (captured in namecheaptest/fixtures/users_address_setDefault.xml).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/set-default/
func (uas *UsersAddressService) SetPrimaryDefaultWithContext(ctx context.Context) (*UsersAddressSetDefaultCommandResponse, error) {
	return uas.setDefault(ctx, PrimaryAddressID)
}

func (uas *UsersAddressService) setDefault(ctx context.Context, addressID int) (*UsersAddressSetDefaultCommandResponse, error) {
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
