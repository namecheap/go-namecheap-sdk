package namecheaptest_test

import (
	"context"
	"testing"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
	"github.com/namecheap/go-namecheap-sdk/v2/namecheaptest"
)

// TestUsersAddFundsFromCapturedResponses replays the sandbox
// createaddfundsrequest and getAddFundsStatus responses captured on
// 2026-11-06: the result element is spelled Createaddfundsrequestresult, so
// the SDK must return the token and URLs rather than a nil result, and a
// CREATED request reports TransactionID 0 and the amount without decimals.
func TestUsersAddFundsFromCapturedResponses(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.users.createaddfundsrequest", "users_createaddfundsrequest")
	srv.StubFixture("namecheap.users.getAddFundsStatus", "users_getAddFundsStatus")
	client := srv.Client()

	created, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), &namecheap.UsersCreateAddFundsRequestArgs{
		PaymentType: namecheap.PaymentTypeCreditcard, Amount: "10.00", ReturnURL: "https://example.com/add-funds/return",
	})
	if err != nil {
		t.Fatalf("CreateAddFundsRequestWithContext() error = %v, want nil", err)
	}
	cr := created.CreateAddFundsRequestResult
	if cr == nil || cr.TokenID == nil {
		t.Fatalf("CreateAddFundsRequestWithContext().CreateAddFundsRequestResult = %+v, want the token", cr)
	}
	const token = "fd935045fec54dc08b8b45a3c99d2262"
	if *cr.TokenID != token {
		t.Errorf("CreateAddFundsRequestWithContext().TokenID = %q, want %q", *cr.TokenID, token)
	}
	if cr.RedirectURL == nil || cr.ReturnURL == nil || *cr.RedirectURL != *cr.ReturnURL {
		t.Errorf("CreateAddFundsRequestWithContext() RedirectURL = %v, ReturnURL = %v, want the same payment URL", cr.RedirectURL, cr.ReturnURL)
	}

	status, err := client.Users.GetAddFundsStatusWithContext(context.Background(), *cr.TokenID)
	if err != nil {
		t.Fatalf("GetAddFundsStatusWithContext(%s) error = %v, want nil", token, err)
	}
	srv.AssertCalled(t, "namecheap.users.getAddFundsStatus", map[string]string{"TokenId": token})
	sr := status.GetAddFundsStatusResult
	if sr == nil {
		t.Fatal("GetAddFundsStatusWithContext().GetAddFundsStatusResult = nil, want the status")
	}
	if sr.Status != namecheap.AddFundsStatusCreated {
		t.Errorf("GetAddFundsStatusWithContext().Status = %q, want %q", sr.Status, namecheap.AddFundsStatusCreated)
	}
	if sr.TransactionID == nil || *sr.TransactionID != 0 {
		t.Errorf("GetAddFundsStatusWithContext().TransactionID = %v, want 0 for a CREATED request", sr.TransactionID)
	}
	if sr.Amount == nil || *sr.Amount != "10" {
		t.Errorf("GetAddFundsStatusWithContext().Amount = %v, want %q (the server drops the decimals)", sr.Amount, "10")
	}
	if sr.ReturnedTokenID == nil || *sr.ReturnedTokenID != token {
		t.Errorf("GetAddFundsStatusWithContext().ReturnedTokenID = %v, want %q", sr.ReturnedTokenID, token)
	}
}

// TestUsersUpdateFromCapturedResponse replays the sandbox users.update response
// captured on 2026-11-06: Success and the UserId attribute.
func TestUsersUpdateFromCapturedResponse(t *testing.T) {
	t.Parallel()
	srv := namecheaptest.NewServer(t)
	srv.StubFixture("namecheap.users.update", "users_update")

	resp, err := srv.Client().Users.UpdateWithContext(context.Background(), &namecheap.UsersUpdateArgs{
		FirstName: "Test", LastName: "Probe", Address1: "1 Test Street", City: "Phoenix", StateProvince: "AZ",
		Zip: "85284", Country: "US", EmailAddress: "user@example.com", Phone: "+1.5555555555",
	})
	if err != nil {
		t.Fatalf("UpdateWithContext() error = %v, want nil", err)
	}
	ur := resp.UserUpdateResult
	if ur == nil || ur.Success == nil || !*ur.Success {
		t.Fatalf("UpdateWithContext().UserUpdateResult = %+v, want Success", ur)
	}
	if ur.UserID == nil || *ur.UserID != "249707" {
		t.Errorf("UpdateWithContext().UserID = %v, want %q", ur.UserID, "249707")
	}
}
