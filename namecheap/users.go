package namecheap

// UsersService groups the namecheap.users.* API commands: pricing and account
// funds (GetPricingWithContext, GetBalancesWithContext), the add-funds flow
// (CreateAddFundsRequestWithContext, GetAddFundsStatusWithContext), and account
// maintenance (ChangePasswordWithContext, UpdateWithContext).
//
// The reseller account-creation surface (namecheap.users.create,
// namecheap.users.login and namecheap.users.resetPassword) is intentionally not
// implemented; see the coverage matrix in README.md for the rationale.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/
type UsersService service
