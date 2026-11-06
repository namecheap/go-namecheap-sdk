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
			<List AddressId="0" AddressName="Primary Address" IsDefault="true" />
			<List AddressId="778" AddressName="Office" IsDefault="false" />
			<List AddressId="779" AddressName="Billing" IsDefault="false" />
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
		assert.Equal(t, 0, *entries[0].AddressID)
		assert.Equal(t, "Primary Address", *entries[0].AddressName)
		mustNotNil(t, entries[0].IsDefault)
		assert.True(t, *entries[0].IsDefault)
		assert.Equal(t, 779, *entries[2].AddressID)
		assert.Equal(t, "Billing", *entries[2].AddressName)
		mustNotNil(t, entries[2].IsDefault)
		assert.False(t, *entries[2].IsDefault)
	})

	t.Run("api error mapped to APIError", func(t *testing.T) {
		t.Parallel()
		client := usersMockClient(t, apiErrorXML(4011103, "API access denied"), nil)
		_, err := client.UsersAddress.GetListWithContext(context.Background())
		assertAPIError(t, err, 4011103)
	})
}
