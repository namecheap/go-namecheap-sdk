package namecheap

// DomainsDNSService includes the following methods:
// DomainsDNSService.GetEmailForwarding - returns email forwarding rules for the requested domain
// DomainsDNSService.GetHosts - retrieves DNS host record settings for the requested domain
// DomainsDNSService.GetList - gets a list of DNS servers associated with the requested domain
// DomainsDNSService.SetCustom - sets domain to use custom DNS servers
// DomainsDNSService.SetDefault - sets domain to use our default DNS servers
// DomainsDNSService.SetEmailForwarding - configures email forwarding rules for the requested domain
// DomainsDNSService.SetHosts - sets DNS host records settings for the requested domain
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-dns/
type DomainsDNSService service
