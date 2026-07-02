package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsersAddressService_GetInfo(t *testing.T) {
	t.Parallel()

	t.Run("success parse and request", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressGetInfoResponse, &sent)

		resp, err := client.UsersAddress.GetInfoWithContext(context.Background(), 777)
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.address.getInfo", sent.Get("Command"))
		assert.Equal(t, "777", sent.Get("AddressId"))

		got := resp.GetAddressInfoResult
		mustNotNil(t, got)
		assert.Equal(t, "Home", *got.AddressName)
		assert.Equal(t, "85284", *got.Zip)
		assert.Equal(t, "NameCheap.com", *got.Organization)
	})

	t.Run("invalid id no http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.UsersAddress.GetInfoWithContext(context.Background(), 0)
		assertInvalidArguments(t, err, "AddressId")
	})

	t.Run("api error mapped to APIError", func(t *testing.T) {
		t.Parallel()
		client := usersMockClient(t, apiErrorXML(4022336, "Failed to retrieve user's address"), nil)
		_, err := client.UsersAddress.GetInfoWithContext(context.Background(), 777)
		assertAPIError(t, err, 4022336)
	})
}
