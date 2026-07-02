package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const changePasswordResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.changePassword">
		<UserChangePasswordResult Success="true" UserID="4242" />
	</CommandResponse>
</ApiResponse>`

func TestUsersService_ChangePassword(t *testing.T) {
	t.Parallel()

	t.Run("old password method", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, changePasswordResponse, &sent)

		resp, err := client.Users.ChangePasswordWithContext(context.Background(), &UsersChangePasswordArgs{
			OldPassword: "oldSecret",
			NewPassword: "newSecret",
		})
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.changePassword", sent.Get("Command"))
		assert.Equal(t, "oldSecret", sent.Get("OldPassword"))
		assert.Equal(t, "newSecret", sent.Get("NewPassword"))
		_, hasReset := sent["ResetCode"]
		assert.False(t, hasReset, "ResetCode must not be sent for the old-password method")

		result := resp.UserChangePasswordResult
		mustNotNil(t, result)
		assert.True(t, *result.Success)
		assert.Equal(t, 4242, *result.UserID)
	})

	t.Run("reset code method", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, changePasswordResponse, &sent)

		_, err := client.Users.ChangePasswordWithContext(context.Background(), &UsersChangePasswordArgs{
			ResetCode:   "reset-123",
			NewPassword: "newSecret",
		})
		mustNoError(t, err)
		assert.Equal(t, "reset-123", sent.Get("ResetCode"))
		assert.Equal(t, "newSecret", sent.Get("NewPassword"))
		_, hasOld := sent["OldPassword"]
		assert.False(t, hasOld, "OldPassword must not be sent for the reset-code method")
	})

	t.Run("nil args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.ChangePasswordWithContext(context.Background(), nil)
		assertInvalidArguments(t, err, "args")
	})

	t.Run("missing new password", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.ChangePasswordWithContext(context.Background(), &UsersChangePasswordArgs{OldPassword: "x"})
		assertInvalidArguments(t, err, "NewPassword")
	})

	t.Run("no credential", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.ChangePasswordWithContext(context.Background(), &UsersChangePasswordArgs{NewPassword: "x"})
		assertInvalidArguments(t, err, "OldPassword", "ResetCode")
	})

	t.Run("both credentials rejected", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.ChangePasswordWithContext(context.Background(), &UsersChangePasswordArgs{
			OldPassword: "x",
			ResetCode:   "y",
			NewPassword: "z",
		})
		assertInvalidArguments(t, err, "OldPassword", "ResetCode")
	})
}
