package namecheap

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const getBalancesResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.getBalances">
		<UserGetBalancesResult Currency="USD" AvailableBalance="123.45" AccountBalance="123.45" EarnedAmount="15.00" WithdrawableAmount="100.00" FundsRequiredForAutoRenew="42.50" />
	</CommandResponse>
</ApiResponse>`

func TestUsersService_GetBalances(t *testing.T) {
	t.Parallel()

	var sent url.Values
	client := usersMockClient(t, getBalancesResponse, &sent)

	resp, err := client.Users.GetBalancesWithContext(context.Background())
	mustNoError(t, err)
	assert.Equal(t, "namecheap.users.getBalances", sent.Get("Command"))

	result := resp.UserGetBalancesResult
	mustNotNil(t, result)
	assert.Equal(t, "USD", result.Currency)

	// 123.45 has no exact binary float representation; assert the exact string is
	// preserved by the Amount type (never parsed to float64).
	assert.Equal(t, Amount("123.45"), result.AvailableBalance)
	assert.Equal(t, "123.45", result.AvailableBalance.String())
	assert.Equal(t, Amount("123.45"), result.AccountBalance)
	assert.Equal(t, Amount("15.00"), result.EarnedAmount)
	assert.Equal(t, Amount("100.00"), result.WithdrawableAmount)
	assert.Equal(t, Amount("42.50"), result.FundsRequiredForAutoRenew)
}

func TestUsersService_GetBalances_APIError(t *testing.T) {
	t.Parallel()

	client := usersMockClient(t, apiErrorXML(4011103, "API Key is invalid or API access has not been enabled"), nil)
	_, err := client.Users.GetBalancesWithContext(context.Background())
	assertAPIError(t, err, 4011103)
}
