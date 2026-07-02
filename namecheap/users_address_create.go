package namecheap

import (
	"context"
	"encoding/xml"
)

// UsersAddressCreateResponse is the raw envelope for
// namecheap.users.address.create.
type UsersAddressCreateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersAddressCreateCommandResponse `xml:"CommandResponse"`
}

// UsersAddressCreateCommandResponse wraps the create result. The doc lists no
// response table for address.create (docs/namecheap-api-v2.md lines 1339-1365);
// the result element carries the new address id and a Success flag on the wire.
type UsersAddressCreateCommandResponse struct {
	AddressCreateResult *UsersAddressCreateResult `xml:"AddressCreateResult"`
}

// UsersAddressCreateResult is the outcome of an address create.
type UsersAddressCreateResult struct {
	// Success reports whether the address was created.
	Success *bool `xml:"Success,attr"`
	// AddressID is the unique integer identifying the new address profile.
	AddressID *int `xml:"AddressID,attr"`
}

// CreateWithContext adds a new entry to the user's address book.
//
// It is a non-idempotent write but not charge-bearing; it uses the standard
// idempotent transport. Every required field is validated before the request is
// sent (see UsersAddressDetails); a violation returns an *InvalidArgumentsError
// listing all missing fields at once.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/create/
func (uas *UsersAddressService) CreateWithContext(ctx context.Context, details *UsersAddressDetails) (*UsersAddressCreateCommandResponse, error) {
	if details == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"details"}, Reason: "details must not be nil"}
	}
	if missing := details.missingFields(); len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{"Command": "namecheap.users.address.create"}
	details.apply(params)

	var response UsersAddressCreateResponse
	_, err := uas.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
