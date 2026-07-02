package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const addressUpdateResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.update">
		<AddressUpdateResult Success="true" AddressID="777" />
	</CommandResponse>
</ApiResponse>`

func TestUsersAddressService_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressUpdateResponse, &sent)

		resp, err := client.UsersAddress.UpdateWithContext(context.Background(), 777, validAddressDetails())
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.address.update", sent.Get("Command"))
		assert.Equal(t, "777", sent.Get("AddressId"))
		assert.Equal(t, "Home", sent.Get("AddressName"))
		assert.Equal(t, "S", sent.Get("StateProvinceChoice"))

		mustNotNil(t, resp.AddressUpdateResult)
		assert.True(t, *resp.AddressUpdateResult.Success)
		assert.Equal(t, 777, *resp.AddressUpdateResult.AddressID)
	})

	t.Run("nil details", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.UsersAddress.UpdateWithContext(context.Background(), 777, nil)
		assertInvalidArguments(t, err, "details")
	})

	t.Run("invalid id and missing fields reported together", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		details := validAddressDetails()
		details.FirstName = ""

		_, err := client.UsersAddress.UpdateWithContext(context.Background(), 0, details)
		assertInvalidArguments(t, err, "AddressId", "FirstName")
	})
}
