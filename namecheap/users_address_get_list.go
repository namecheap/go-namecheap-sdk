package namecheap

import (
	"context"
	"encoding/xml"
)

// UsersAddressGetListResponse is the raw envelope for
// namecheap.users.address.getList.
type UsersAddressGetListResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersAddressGetListCommandResponse `xml:"CommandResponse"`
}

// UsersAddressGetListCommandResponse wraps the list of address entries.
//
// Unlike domains.getList, the doc's address.getList
// (docs/namecheap-api-v2.md lines 1412-1428) defines no request parameters and no
// paging block, so none is modeled here: the response is a flat list of every
// address on the account.
type UsersAddressGetListCommandResponse struct {
	AddressGetListResult *[]UsersAddressListEntry `xml:"AddressGetListResult>List"`
}

// UsersAddressListEntry is one address-book entry in the list. Field names follow
// the getList response table in docs/namecheap-api-v2.md (lines 1424-1425).
type UsersAddressListEntry struct {
	// AddressID is the unique integer representing the address profile.
	AddressID *int `xml:"AddressId,attr"`
	// AddressName is the name of the address profile.
	AddressName *string `xml:"AddressName,attr"`
}

// GetListWithContext returns every address-book entry (id and name) on the
// account.
//
// It is a read-only, idempotent call and takes no parameters. Per the doc the
// response is not paged.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/get-list/
func (uas *UsersAddressService) GetListWithContext(ctx context.Context) (*UsersAddressGetListCommandResponse, error) {
	var response UsersAddressGetListResponse
	params := map[string]string{
		"Command": "namecheap.users.address.getList",
	}

	_, err := uas.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
