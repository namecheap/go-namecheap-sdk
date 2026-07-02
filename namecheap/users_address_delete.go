package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// UsersAddressDeleteResponse is the raw envelope for
// namecheap.users.address.delete.
type UsersAddressDeleteResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersAddressDeleteCommandResponse `xml:"CommandResponse"`
}

// UsersAddressDeleteCommandResponse wraps the delete result.
type UsersAddressDeleteCommandResponse struct {
	AddressDeleteResult *UsersAddressDeleteResult `xml:"AddressDeleteResult"`
}

// UsersAddressDeleteResult is the outcome of an address delete. Field names
// follow the response table in docs/namecheap-api-v2.md (lines 1383-1387).
type UsersAddressDeleteResult struct {
	// Success reports whether the address was deleted.
	Success *bool `xml:"Success,attr"`
	// ProfileID is the unique integer representing the address profile.
	ProfileID *int `xml:"ProfileID,attr"`
	// Username is the account the address belonged to.
	Username *string `xml:"Username,attr"`
}

// DeleteWithContext removes an address-book entry by id.
//
// It is a non-idempotent write but not charge-bearing; it uses the standard
// idempotent transport. addressID must be positive.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/delete/
func (uas *UsersAddressService) DeleteWithContext(ctx context.Context, addressID int) (*UsersAddressDeleteCommandResponse, error) {
	if addressID <= 0 {
		return nil, &InvalidArgumentsError{Fields: []string{"AddressId"}}
	}

	params := map[string]string{
		"Command":   "namecheap.users.address.delete",
		"AddressId": strconv.Itoa(addressID),
	}

	var response UsersAddressDeleteResponse
	_, err := uas.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
