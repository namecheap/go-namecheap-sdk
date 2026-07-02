package namecheap

import (
	"context"
	"encoding/xml"
)

// UsersGetBalancesResponse is the raw envelope for namecheap.users.getBalances.
type UsersGetBalancesResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersGetBalancesCommandResponse `xml:"CommandResponse"`
}

// UsersGetBalancesCommandResponse wraps the getBalances result.
type UsersGetBalancesCommandResponse struct {
	UserGetBalancesResult *UsersGetBalancesResult `xml:"UserGetBalancesResult"`
}

// UsersGetBalancesResult holds the account funds. Field names follow the
// getBalances response table in docs/namecheap-api-v2.md (lines 1146-1153). Every
// monetary field is an Amount (an exact decimal string) so a balance used to gate
// a charge is never mangled by binary floating point; Currency is a plain string.
type UsersGetBalancesResult struct {
	// Currency is the currency the amounts are listed in, e.g. "USD".
	Currency string `xml:"Currency,attr"`
	// AvailableBalance is the total amount available in the account.
	AvailableBalance Amount `xml:"AvailableBalance,attr"`
	// AccountBalance is the total amount in the account (per the doc, the same as
	// AvailableBalance).
	AccountBalance Amount `xml:"AccountBalance,attr"`
	// EarnedAmount is the amount earned from Marketplace sales.
	EarnedAmount Amount `xml:"EarnedAmount,attr"`
	// WithdrawableAmount is the amount available for withdrawal.
	WithdrawableAmount Amount `xml:"WithdrawableAmount,attr"`
	// FundsRequiredForAutoRenew is the amount required for auto-renewal.
	FundsRequiredForAutoRenew Amount `xml:"FundsRequiredForAutoRenew,attr"`
}

// GetBalancesWithContext returns the funds in the user's account, decimal-safe.
//
// It is a read-only, idempotent call and takes no parameters. It is the
// precondition check for every money-bearing operation: read the balance and
// gate a bulk renew/create/transfer on it before spending.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/get-balances/
func (us *UsersService) GetBalancesWithContext(ctx context.Context) (*UsersGetBalancesCommandResponse, error) {
	var response UsersGetBalancesResponse
	params := map[string]string{
		"Command": "namecheap.users.getBalances",
	}

	_, err := us.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
