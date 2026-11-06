package namecheap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const createAddFundsResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.createaddfundsrequest">
		<Createaddfundsrequestresult TokenID="1234567890" ReturnURL="https://www.sandbox.namecheap.com/myaccount/addfunds/Payment.aspx?tokenid=1234567890" RedirectURL="https://www.sandbox.namecheap.com/myaccount/addfunds/Payment.aspx?tokenid=1234567890" />
	</CommandResponse>
</ApiResponse>`

const getAddFundsStatusResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.getAddFundsStatus">
		<GetAddFundsStatusResult TransactionID="998877" Amount="50.00" Status="COMPLETED" RequestedTokenId="1234567890" ReturnedTokenId="1234567890" />
	</CommandResponse>
</ApiResponse>`

func validAddFundsArgs() *UsersCreateAddFundsRequestArgs {
	return &UsersCreateAddFundsRequestArgs{
		PaymentType: PaymentTypeCreditcard,
		Amount:      Amount("50.00"),
		ReturnURL:   "https://example.com/done",
	}
}

func TestUsersService_CreateAddFundsRequest(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, createAddFundsResponse, &sent)

		resp, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), validAddFundsArgs())
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.createaddfundsrequest", sent.Get("Command"))
		assert.Equal(t, "Creditcard", sent.Get("PaymentType"))
		assert.Equal(t, "50.00", sent.Get("Amount"))
		assert.Equal(t, "https://example.com/done", sent.Get("ReturnUrl"))
		// The target account is the authenticated user, set by the transport.
		assert.Equal(t, ncUserName, sent.Get("Username"))

		result := resp.CreateAddFundsRequestResult
		mustNotNil(t, result)
		assert.Equal(t, "1234567890", *result.TokenID)
		mustNotNil(t, result.RedirectURL)
		assert.Equal(t, *result.RedirectURL, *result.ReturnURL, "the server sends the payment page URL in both")
		assert.Equal(t, "https://www.sandbox.namecheap.com/myaccount/addfunds/Payment.aspx?tokenid=1234567890", *result.RedirectURL)
	})

	// Production has no capture; the documented spelling must not lose the token.
	t.Run("documented result spelling", func(t *testing.T) {
		t.Parallel()
		body := strings.Replace(createAddFundsResponse, "<Createaddfundsrequestresult ", "<CreateAddFundsRequestResult ", 1)
		client := usersMockClient(t, body, nil)

		resp, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), validAddFundsArgs())
		mustNoError(t, err)
		mustNotNil(t, resp.CreateAddFundsRequestResult)
		mustNotNil(t, resp.CreateAddFundsRequestResult.TokenID)
		assert.Equal(t, "1234567890", *resp.CreateAddFundsRequestResult.TokenID)
	})

	t.Run("nil args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), nil)
		assertInvalidArguments(t, err, "args")
	})

	t.Run("missing required fields reported all at once no http", func(t *testing.T) {
		t.Parallel()
		var called int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			atomic.AddInt32(&called, 1)
		}))
		defer server.Close()

		client := setupClient(nil)
		client.BaseURL = server.URL

		_, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), &UsersCreateAddFundsRequestArgs{})
		assertInvalidArguments(t, err, "PaymentType", "Amount", "ReturnUrl")
		assert.Equal(t, int32(0), atomic.LoadInt32(&called), "no charge-bearing call may happen when validation fails")
	})
}

// TestUsersService_CreateAddFundsRequest_NonIdempotent proves the charge-bearing
// createaddfundsrequest is NOT retried on an ambiguous retryable server error —
// exactly one attempt is made — mirroring the #114 idempotency test for
// domains.create.
func TestUsersService_CreateAddFundsRequest_NonIdempotent(t *testing.T) {
	t.Parallel()

	t.Run("server error single attempt", func(t *testing.T) {
		t.Parallel()
		var calls int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			_, _ = w.Write([]byte(apiErrorXML(5050900, "Server exception")))
		}))
		defer server.Close()

		client := newResilienceClient(server.URL, func(o *ClientOptions) {
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), validAddFundsArgs())
		assertAPIError(t, err, 5050900)
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "an ambiguous server error must not be retried on a charge-bearing call")
		assert.NotContains(t, err.Error(), "after")
	})

	t.Run("transport timeout single attempt", func(t *testing.T) {
		t.Parallel()
		rt := &countingErrRT{err: timeoutError{}}
		client := newResilienceClient("http://127.0.0.1:0", func(o *ClientOptions) {
			o.Transport = rt
			o.RateLimit = &RateLimitOptions{Disabled: true}
			o.Retry = &RetryOptions{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
		})

		_, err := client.Users.CreateAddFundsRequestWithContext(context.Background(), validAddFundsArgs())
		assert.Error(t, err)
		assert.Equal(t, 1, rt.count(), "an ambiguous timeout must not be retried on a charge-bearing call")
	})
}

func TestUsersService_GetAddFundsStatus(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, getAddFundsStatusResponse, &sent)

		resp, err := client.Users.GetAddFundsStatusWithContext(context.Background(), "1234567890")
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.getAddFundsStatus", sent.Get("Command"))
		assert.Equal(t, "1234567890", sent.Get("TokenId"))

		result := resp.GetAddFundsStatusResult
		mustNotNil(t, result)
		assert.Equal(t, 998877, *result.TransactionID)
		assert.Equal(t, Amount("50.00"), *result.Amount)
		assert.Equal(t, "1234567890", *result.RequestedTokenID)
		assert.Equal(t, "1234567890", *result.ReturnedTokenID)
		assert.Equal(t, AddFundsStatusCompleted, result.Status)
	})

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.GetAddFundsStatusWithContext(context.Background(), "")
		assertInvalidArguments(t, err, "TokenId")
	})
}

func TestAddFundsStatusConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, AddFundsStatus("CREATED"), AddFundsStatusCreated)
	assert.Equal(t, AddFundsStatus("SUBMITTED"), AddFundsStatusSubmitted)
	assert.Equal(t, AddFundsStatus("COMPLETED"), AddFundsStatusCompleted)
	assert.Equal(t, AddFundsStatus("FAILED"), AddFundsStatusFailed)
	assert.Equal(t, AddFundsStatus("EXPIRED"), AddFundsStatusExpired)
}
