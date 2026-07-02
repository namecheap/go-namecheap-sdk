package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const usersUpdateResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.update">
		<UserUpdateResult Success="true" />
	</CommandResponse>
</ApiResponse>`

func validUpdateArgs() *UsersUpdateArgs {
	return &UsersUpdateArgs{
		FirstName:     "John",
		LastName:      "Smith",
		Address1:      "8939 S.cross Blvd",
		City:          "Phoenix",
		StateProvince: "AZ",
		Zip:           "85284",
		Country:       "US",
		EmailAddress:  "john@example.com",
		Phone:         "+1.6613102107",
	}
}

func TestUsersService_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, usersUpdateResponse, &sent)

		args := validUpdateArgs()
		args.Organization = "NameCheap.com"
		args.PhoneExt = "42"

		resp, err := client.Users.UpdateWithContext(context.Background(), args)
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.update", sent.Get("Command"))
		assert.Equal(t, "John", sent.Get("FirstName"))
		assert.Equal(t, "85284", sent.Get("Zip"))
		assert.Equal(t, "NameCheap.com", sent.Get("Organization"))
		assert.Equal(t, "42", sent.Get("PhoneExt"))
		// An unset optional is never sent as a blank value.
		_, hasFax := sent["Fax"]
		assert.False(t, hasFax)

		mustNotNil(t, resp.UserUpdateResult)
		assert.True(t, *resp.UserUpdateResult.Success)
	})

	t.Run("nil args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.UpdateWithContext(context.Background(), nil)
		assertInvalidArguments(t, err, "args")
	})

	t.Run("missing required fields reported all at once", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		args := validUpdateArgs()
		args.FirstName = ""
		args.EmailAddress = ""
		args.Zip = ""

		_, err := client.Users.UpdateWithContext(context.Background(), args)
		assertInvalidArguments(t, err, "FirstName", "EmailAddress", "Zip")
	})
}
