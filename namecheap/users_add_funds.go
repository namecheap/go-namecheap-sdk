package namecheap

import (
	"context"
	"encoding/xml"
)

// AddFundsStatus is the state of an add-funds request as reported by
// namecheap.users.getAddFundsStatus. The documented values are listed in
// docs/namecheap-api-v2.md (line 1256).
type AddFundsStatus string

// Documented add-funds statuses (docs/namecheap-api-v2.md line 1256).
const (
	// AddFundsStatusCreated means the request was created but not yet submitted.
	AddFundsStatusCreated AddFundsStatus = "CREATED"
	// AddFundsStatusSubmitted means the payment was submitted and is processing.
	AddFundsStatusSubmitted AddFundsStatus = "SUBMITTED"
	// AddFundsStatusCompleted means the funds were added successfully.
	AddFundsStatusCompleted AddFundsStatus = "COMPLETED"
	// AddFundsStatusFailed means the request failed.
	AddFundsStatusFailed AddFundsStatus = "FAILED"
	// AddFundsStatusExpired means the request expired before completion.
	AddFundsStatusExpired AddFundsStatus = "EXPIRED"
)

// PaymentTypeCreditcard is the only documented PaymentType for
// createaddfundsrequest (docs/namecheap-api-v2.md line 1224).
const PaymentTypeCreditcard = "Creditcard"

// UsersCreateAddFundsRequestResponse is the raw envelope for
// namecheap.users.createaddfundsrequest.
type UsersCreateAddFundsRequestResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersCreateAddFundsRequestCommandResponse `xml:"CommandResponse"`
}

// UsersCreateAddFundsRequestCommandResponse wraps the createaddfundsrequest
// result.
type UsersCreateAddFundsRequestCommandResponse struct {
	CreateAddFundsRequestResult *UsersCreateAddFundsRequestResult `xml:"CreateAddFundsRequestResult"`
}

// UsersCreateAddFundsRequestResult is the outcome of an add-funds request. Field
// names follow the response table in docs/namecheap-api-v2.md (lines 1230-1234).
type UsersCreateAddFundsRequestResult struct {
	// TokenID is the unique ID used to redirect the user to the add-funds page and
	// to poll GetAddFundsStatusWithContext.
	TokenID *string `xml:"TokenID,attr"`
	// RedirectURL is the URL for submitting credit-card details.
	RedirectURL *string `xml:"RedirectURL,attr"`
	// ReturnURL is the URL the user is redirected to after payment.
	ReturnURL *string `xml:"ReturnURL,attr"`
}

// UsersCreateAddFundsRequestArgs are the arguments for
// CreateAddFundsRequestWithContext. Field names and required status follow the
// request table in docs/namecheap-api-v2.md (lines 1221-1226).
//
// The doc's "Username" parameter is not exposed here: the transport reserves the
// "Username" request parameter for the authenticated account (ClientOptions.
// UserName) and sets it on every call, so funds are always added to the
// authenticated user's account and a separate field would be silently overwritten.
type UsersCreateAddFundsRequestArgs struct {
	// PaymentType is the payment method. Required; the only documented value is
	// PaymentTypeCreditcard.
	PaymentType string
	// Amount is the amount to add, as an exact decimal string (see Amount).
	// Required.
	Amount Amount
	// ReturnURL is the URL to redirect the user to after payment. Required. It is
	// sent as the "ReturnUrl" request parameter (the response echoes it back as
	// "ReturnURL").
	ReturnURL string
}

// CreateAddFundsRequestWithContext creates a credit-card add-funds request.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could create a duplicate
// funding request / double charge), so the caller must reconcile such failures
// via GetAddFundsStatusWithContext or the account history. Only Namecheap's
// pre-execution HTTP 405 rate-limit signal is retried. This mirrors the money
// rule established for domains create/renew in #114.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/create-add-funds-request/
func (us *UsersService) CreateAddFundsRequestWithContext(ctx context.Context, args *UsersCreateAddFundsRequestArgs) (*UsersCreateAddFundsRequestCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response UsersCreateAddFundsRequestResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := us.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing required field at once.
func (a *UsersCreateAddFundsRequestArgs) validate() error {
	missing := make([]string, 0, 3)
	if a.PaymentType == "" {
		missing = append(missing, "PaymentType")
	}
	if a.Amount == "" {
		missing = append(missing, "Amount")
	}
	if a.ReturnURL == "" {
		missing = append(missing, "ReturnUrl")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map. The "Username"
// parameter is supplied by the transport from the authenticated credentials.
func (a *UsersCreateAddFundsRequestArgs) params() map[string]string {
	return map[string]string{
		"Command":     "namecheap.users.createaddfundsrequest",
		"PaymentType": a.PaymentType,
		"Amount":      a.Amount.String(),
		"ReturnUrl":   a.ReturnURL,
	}
}

// UsersGetAddFundsStatusResponse is the raw envelope for
// namecheap.users.getAddFundsStatus.
type UsersGetAddFundsStatusResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersGetAddFundsStatusCommandResponse `xml:"CommandResponse"`
}

// UsersGetAddFundsStatusCommandResponse wraps the getAddFundsStatus result.
type UsersGetAddFundsStatusCommandResponse struct {
	GetAddFundsStatusResult *UsersGetAddFundsStatusResult `xml:"GetAddFundsStatusResult"`
}

// UsersGetAddFundsStatusResult is the status of an add-funds request. Field names
// follow the response table in docs/namecheap-api-v2.md (lines 1252-1256).
type UsersGetAddFundsStatusResult struct {
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
	// Amount is the amount added, as an exact decimal string (see Amount).
	Amount *Amount `xml:"Amount,attr"`
	// Status is the request status, one of the AddFundsStatus constants.
	Status AddFundsStatus `xml:"Status,attr"`
}

// GetAddFundsStatusWithContext returns the status of an add-funds request
// previously created with CreateAddFundsRequestWithContext.
//
// It is a read-only, idempotent call: unlike the create call it is safe to retry,
// and it is the reconciliation path for an ambiguous create failure.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/get-add-funds-status/
func (us *UsersService) GetAddFundsStatusWithContext(ctx context.Context, tokenID string) (*UsersGetAddFundsStatusCommandResponse, error) {
	if tokenID == "" {
		return nil, &InvalidArgumentsError{Fields: []string{"TokenId"}}
	}

	var response UsersGetAddFundsStatusResponse
	params := map[string]string{
		"Command": "namecheap.users.getAddFundsStatus",
		"TokenId": tokenID,
	}

	_, err := us.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
