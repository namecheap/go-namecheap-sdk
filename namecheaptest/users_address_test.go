package namecheaptest_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
	"github.com/namecheap/go-namecheap-sdk/v2/namecheaptest"
)

// TestUsersAddressDefaultFromCapturedResponses replays the captured sandbox
// getList and getInfo responses (#212): getList carries the IsDefault flag for
// every entry, exactly one of them the default, the default's id (the primary
// address is 0) is accepted by GetInfo, and getInfo's Default_YN agrees.
func TestUsersAddressDefaultFromCapturedResponses(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.users.address.getList", "users_address_getList")
	srv.StubFixture("namecheap.users.address.getInfo", "users_address_getInfo")
	client := srv.Client()

	entries, err := client.UsersAddress.ListAllSlice(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	var defaults []int
	for _, e := range entries {
		if !assert.NotNil(t, e.AddressID, "getList entry AddressId") || !assert.NotNil(t, e.IsDefault, "getList entry IsDefault") {
			return
		}
		if *e.IsDefault {
			defaults = append(defaults, *e.AddressID)
		}
	}
	if !assert.Len(t, defaults, 1, "getList entries with IsDefault=true") {
		return
	}
	defaultID := defaults[0]

	info, err := client.UsersAddress.GetInfoWithContext(context.Background(), defaultID)
	if !assert.NoError(t, err) {
		return
	}
	srv.AssertCalled(t, "namecheap.users.address.getInfo", map[string]string{
		"AddressId": strconv.Itoa(defaultID),
	})
	got := info.GetAddressInfoResult
	if !assert.NotNil(t, got) {
		return
	}
	if assert.NotNil(t, got.AddressID) {
		assert.Equal(t, defaultID, *got.AddressID)
	}
	if assert.NotNil(t, got.DefaultYN) {
		assert.True(t, *got.DefaultYN)
	}
	assert.NotNil(t, got.UserName)
}

// TestUsersAddressWritesFromCapturedResponses replays the sandbox create,
// update, setDefault and delete responses captured on 2026-11-03 for #212: the
// server spells the ids AddressId and ProfileId, so the SDK must return them,
// not nil. setDefault is replayed through SetPrimaryDefaultWithContext, which
// sends AddressId 0 to make the primary address the default again.
func TestUsersAddressWritesFromCapturedResponses(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.users.address.create", "users_address_create")
	srv.StubFixture("namecheap.users.address.update", "users_address_update")
	srv.StubFixture("namecheap.users.address.setDefault", "users_address_setDefault")
	srv.StubFixture("namecheap.users.address.delete", "users_address_delete")
	client := srv.Client()
	ctx := context.Background()
	details := &namecheap.UsersAddressDetails{
		AddressName: "sdk-212-probe", DefaultYN: namecheap.Bool(false), EmailAddress: "sdk-probe@example.com",
		FirstName: "Test", LastName: "Probe", Address1: "1 Test Street", City: "Phoenix",
		StateProvince: "AZ", StateProvinceChoice: "S", Zip: "85284", Country: "US", Phone: "+1.5555555555",
	}

	created, err := client.UsersAddress.CreateWithContext(ctx, details)
	if !assert.NoError(t, err) || !assert.NotNil(t, created.AddressCreateResult) {
		return
	}
	cr := created.AddressCreateResult
	if !assert.NotNil(t, cr.AddressID) {
		return
	}
	assert.Equal(t, 149237, *cr.AddressID)
	if assert.NotNil(t, cr.AddressName) {
		assert.Equal(t, "sdk-212-probe", *cr.AddressName)
	}

	updated, err := client.UsersAddress.UpdateWithContext(ctx, 149238, details)
	if assert.NoError(t, err) && assert.NotNil(t, updated.AddressUpdateResult) {
		ur := updated.AddressUpdateResult
		if assert.NotNil(t, ur.AddressID) {
			assert.Equal(t, 149238, *ur.AddressID)
		}
		if assert.NotNil(t, ur.AddressName) {
			assert.Equal(t, "sdk-212-probe2", *ur.AddressName)
		}
	}

	set, err := client.UsersAddress.SetPrimaryDefaultWithContext(ctx)
	if assert.NoError(t, err) && assert.NotNil(t, set.AddressSetDefaultResult) {
		srv.AssertCalled(t, "namecheap.users.address.setDefault", map[string]string{"AddressId": "0"})
		sr := set.AddressSetDefaultResult
		if assert.NotNil(t, sr.Success) {
			assert.True(t, *sr.Success)
		}
		if assert.NotNil(t, sr.AddressID) {
			assert.Equal(t, namecheap.PrimaryAddressID, *sr.AddressID)
		}
	}

	deleted, err := client.UsersAddress.DeleteWithContext(ctx, *cr.AddressID)
	if assert.NoError(t, err) && assert.NotNil(t, deleted.AddressDeleteResult) {
		if dr := deleted.AddressDeleteResult; assert.NotNil(t, dr.ProfileID) {
			assert.Equal(t, *cr.AddressID, *dr.ProfileID)
		}
	}
}
