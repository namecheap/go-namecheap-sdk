package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const addressGetListResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.getList">
		<AddressGetListResult>
			<List AddressId="777" AddressName="Home" />
			<List AddressId="778" AddressName="Office" />
			<List AddressId="779" AddressName="Billing" />
		</AddressGetListResult>
	</CommandResponse>
</ApiResponse>`

func TestUsersAddressService_GetList(t *testing.T) {
	t.Parallel()

	t.Run("success parses all entries", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressGetListResponse, &sent)

		resp, err := client.UsersAddress.GetListWithContext(context.Background())
		mustNoError(t, err)
		assert.Equal(t, "namecheap.users.address.getList", sent.Get("Command"))

		mustNotNil(t, resp.AddressGetListResult)
		entries := *resp.AddressGetListResult
		mustLen(t, entries, 3)
		assert.Equal(t, 777, *entries[0].AddressID)
		assert.Equal(t, "Home", *entries[0].AddressName)
		assert.Equal(t, 779, *entries[2].AddressID)
		assert.Equal(t, "Billing", *entries[2].AddressName)
	})

	t.Run("api error mapped to APIError", func(t *testing.T) {
		t.Parallel()
		client := usersMockClient(t, apiErrorXML(4011103, "API access denied"), nil)
		_, err := client.UsersAddress.GetListWithContext(context.Background())
		assertAPIError(t, err, 4011103)
	})
}
