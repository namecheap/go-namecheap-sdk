package namecheap

// DomainsService groups the namecheap.domains.* API commands: listing and
// inspecting domains (GetListWithContext, GetInfoWithContext, CheckWithContext,
// GetTldListWithContext), registration lifecycle (CreateWithContext,
// RenewWithContext, ReactivateWithContext), contacts (GetContactsWithContext,
// SetContactsWithContext) and the registrar lock (GetRegistrarLockWithContext,
// SetRegistrarLockWithContext).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/
type DomainsService service
