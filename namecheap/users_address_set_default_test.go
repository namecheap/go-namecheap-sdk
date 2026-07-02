package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const addressSetDefaultResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.setDefault">
		<AddressSetDefaultResult Success="true" AddressID="778" />
	</CommandResponse>
</ApiResponse>`

func TestUsersAddressService_SetDefault(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressSetDefaultResponse, &sent)

		resp, err := client.UsersAddress.SetDefaultWithContext(context.Background(), 778)
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.address.setDefault", sent.Get("Command"))
		assert.Equal(t, "778", sent.Get("AddressId"))

		mustNotNil(t, resp.AddressSetDefaultResult)
		assert.True(t, *resp.AddressSetDefaultResult.Success)
		assert.Equal(t, 778, *resp.AddressSetDefaultResult.AddressID)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.UsersAddress.SetDefaultWithContext(context.Background(), -1)
		assertInvalidArguments(t, err, "AddressId")
	})
}
