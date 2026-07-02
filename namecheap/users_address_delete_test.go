package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const addressDeleteResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.delete">
		<AddressDeleteResult Success="true" ProfileID="777" Username="testuser" />
	</CommandResponse>
</ApiResponse>`

func TestUsersAddressService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressDeleteResponse, &sent)

		resp, err := client.UsersAddress.DeleteWithContext(context.Background(), 777)
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.address.delete", sent.Get("Command"))
		assert.Equal(t, "777", sent.Get("AddressId"))

		mustNotNil(t, resp.AddressDeleteResult)
		assert.True(t, *resp.AddressDeleteResult.Success)
		assert.Equal(t, 777, *resp.AddressDeleteResult.ProfileID)
		assert.Equal(t, "testuser", *resp.AddressDeleteResult.Username)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.UsersAddress.DeleteWithContext(context.Background(), 0)
		assertInvalidArguments(t, err, "AddressId")
	})
}
