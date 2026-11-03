package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// UsersAddressUpdateResponse is the raw envelope for
// namecheap.users.address.update.
type UsersAddressUpdateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersAddressUpdateCommandResponse `xml:"CommandResponse"`
}

// UsersAddressUpdateCommandResponse wraps the update result.
type UsersAddressUpdateCommandResponse struct {
	AddressUpdateResult *UsersAddressUpdateResult `xml:"AddressUpdateResult"`
}

// UsersAddressUpdateResult is the outcome of an address update. Field names
// follow the captured sandbox response in
// namecheaptest/fixtures/users_address_update.xml.
type UsersAddressUpdateResult struct {
	// Success reports whether the address was updated.
	Success *bool `xml:"Success,attr"`
	// AddressID is the unique integer identifying the updated address profile.
	// The response attribute is AddressId.
	AddressID *int `xml:"AddressId,attr"`
	// AddressName is the name of the updated address profile.
	AddressName *string `xml:"AddressName,attr"`
}

// UpdateWithContext updates an existing address-book entry identified by
// addressID with the given details.
//
// It is a non-idempotent write but not charge-bearing; it uses the standard
// idempotent transport. addressID must be positive and every required field of
// details is validated before the request is sent (see UsersAddressDetails).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/update/
func (uas *UsersAddressService) UpdateWithContext(ctx context.Context, addressID int, details *UsersAddressDetails) (*UsersAddressUpdateCommandResponse, error) {
	if details == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"details"}, Reason: "details must not be nil"}
	}
	missing := make([]string, 0, len(details.requiredAddressFields())+1)
	if addressID <= 0 {
		missing = append(missing, "AddressId")
	}
	missing = append(missing, details.missingFields()...)
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	params := map[string]string{
		"Command":   "namecheap.users.address.update",
		"AddressId": strconv.Itoa(addressID),
	}
	details.apply(params)

	var response UsersAddressUpdateResponse
	_, err := uas.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
