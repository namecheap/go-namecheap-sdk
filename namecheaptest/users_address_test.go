package namecheaptest_test

import (
	"context"
	"strconv"
	"testing"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
	"github.com/namecheap/go-namecheap-sdk/v2/namecheaptest"
)

// TestUsersAddressDefaultFromCapturedResponses replays the captured sandbox
// getList and getInfo responses (#212): the default entry is found through
// getList's IsDefault, its id (the primary address is 0) is accepted by
// GetInfo, and getInfo's Default_YN agrees with the list.
func TestUsersAddressDefaultFromCapturedResponses(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.users.address.getList", "users_address_getList")
	srv.StubFixture("namecheap.users.address.getInfo", "users_address_getInfo")
	client := srv.Client()

	entries, err := client.UsersAddress.ListAllSlice(context.Background())
	if err != nil {
		t.Fatalf("ListAllSlice() error = %v, want nil", err)
	}
	var defaultID *int
	for _, e := range entries {
		if e.AddressID == nil {
			t.Fatalf("getList entry %v: AddressID = nil, want the AddressId attribute", e.AddressName)
		}
		if e.IsDefault == nil {
			t.Fatalf("getList entry %d: IsDefault = nil, want the IsDefault attribute", *e.AddressID)
		}
		if *e.IsDefault {
			if defaultID != nil {
				t.Fatalf("getList: entries %d and %d are both default, want one", *defaultID, *e.AddressID)
			}
			defaultID = e.AddressID
		}
	}
	if defaultID == nil {
		t.Fatal("getList: no entry has IsDefault=true, want one")
	}

	info, err := client.UsersAddress.GetInfoWithContext(context.Background(), *defaultID)
	if err != nil {
		t.Fatalf("GetInfoWithContext(%d) error = %v, want nil", *defaultID, err)
	}
	srv.AssertCalled(t, "namecheap.users.address.getInfo", map[string]string{
		"AddressId": strconv.Itoa(*defaultID),
	})
	got := info.GetAddressInfoResult
	if got == nil {
		t.Fatalf("GetInfoWithContext(%d).GetAddressInfoResult = nil, want the address", *defaultID)
	}
	if got.AddressID == nil || *got.AddressID != *defaultID {
		t.Errorf("GetInfoWithContext(%d).AddressID = %v, want %d", *defaultID, got.AddressID, *defaultID)
	}
	if got.DefaultYN == nil || !*got.DefaultYN {
		t.Errorf("GetInfoWithContext(%d).DefaultYN = %v, want true", *defaultID, got.DefaultYN)
	}
}

// TestUsersAddressWritesFromCapturedResponses replays the sandbox create,
// update, setDefault and delete responses captured on 2026-11-03 for #212: the
// server spells the ids AddressId and ProfileId, so the SDK must return them,
// not nil. setDefault is replayed with AddressId 0, which the server accepts to
// make the primary address the default again.
func TestUsersAddressWritesFromCapturedResponses(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.users.address.create", "users_address_create")
	srv.StubFixture("namecheap.users.address.update", "users_address_update")
	srv.StubFixture("namecheap.users.address.setDefault", "users_address_setDefault")
	srv.StubFixture("namecheap.users.address.delete", "users_address_delete")
	client := srv.Client()
	details := &namecheap.UsersAddressDetails{
		AddressName: "sdk-212-probe", DefaultYN: namecheap.Bool(false), EmailAddress: "sdk-probe@example.com",
		FirstName: "Test", LastName: "Probe", Address1: "1 Test Street", City: "Phoenix",
		StateProvince: "AZ", StateProvinceChoice: "S", Zip: "85284", Country: "US", Phone: "+1.5555555555",
	}

	created, err := client.UsersAddress.CreateWithContext(context.Background(), details)
	if err != nil {
		t.Fatalf("CreateWithContext() error = %v, want nil", err)
	}
	cr := created.AddressCreateResult
	if cr == nil || cr.AddressID == nil {
		t.Fatalf("CreateWithContext().AddressCreateResult = %+v, want the new AddressID", cr)
	}
	if *cr.AddressID != 149237 {
		t.Errorf("CreateWithContext().AddressID = %d, want 149237", *cr.AddressID)
	}
	if cr.AddressName == nil || *cr.AddressName != "sdk-212-probe" {
		t.Errorf("CreateWithContext().AddressName = %v, want %q", cr.AddressName, "sdk-212-probe")
	}

	updated, err := client.UsersAddress.UpdateWithContext(context.Background(), 149238, details)
	if err != nil {
		t.Fatalf("UpdateWithContext(149238) error = %v, want nil", err)
	}
	ur := updated.AddressUpdateResult
	if ur == nil || ur.AddressID == nil || *ur.AddressID != 149238 {
		t.Errorf("UpdateWithContext(149238).AddressUpdateResult = %+v, want AddressID 149238", ur)
	}
	if ur != nil && (ur.AddressName == nil || *ur.AddressName != "sdk-212-probe2") {
		t.Errorf("UpdateWithContext(149238).AddressName = %v, want %q", ur.AddressName, "sdk-212-probe2")
	}

	set, err := client.UsersAddress.SetDefaultWithContext(context.Background(), 0)
	if err != nil {
		t.Fatalf("SetDefaultWithContext(0) error = %v, want nil", err)
	}
	srv.AssertCalled(t, "namecheap.users.address.setDefault", map[string]string{"AddressId": "0"})
	sr := set.AddressSetDefaultResult
	if sr == nil || sr.Success == nil || !*sr.Success || sr.AddressID == nil || *sr.AddressID != 0 {
		t.Errorf("SetDefaultWithContext(0).AddressSetDefaultResult = %+v, want Success and AddressID 0", sr)
	}

	deleted, err := client.UsersAddress.DeleteWithContext(context.Background(), *cr.AddressID)
	if err != nil {
		t.Fatalf("DeleteWithContext(%d) error = %v, want nil", *cr.AddressID, err)
	}
	dr := deleted.AddressDeleteResult
	if dr == nil || dr.ProfileID == nil || *dr.ProfileID != *cr.AddressID {
		t.Fatalf("DeleteWithContext(%d).AddressDeleteResult = %+v, want ProfileID %d", *cr.AddressID, dr, *cr.AddressID)
	}
}
