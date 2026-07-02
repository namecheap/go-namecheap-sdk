// Package namecheap provides a Go client for the Namecheap API.
//
// Construct a Client with NewClient and call the service methods on
// Client.Domains, Client.DomainsDNS and Client.DomainsNS. Every call takes a
// context.Context as its first argument; cancelling it aborts the in-flight
// HTTP request, any pending retry sleep, and waiting on the internal retry lock.
//
// # Error handling
//
// When the API rejects a call (envelope Status="ERROR") the method returns a
// typed *APIError carrying the numeric code, the server message and the failing
// command. Responses with several <Error> entries return an errors.Join of
// *APIError values. Malformed responses return a *ParseError; transport and
// context failures propagate unwrapped. All three are distinguishable, so a
// consumer can tell an API rejection from an SDK/parse fault from an outage.
//
// Read the structured code with errors.As:
//
//	resp, err := client.Domains.GetInfoWithContext(ctx, "example.com")
//	if err != nil {
//	    var apiErr *namecheap.APIError
//	    if errors.As(err, &apiErr) {
//	        log.Printf("code=%d message=%q command=%q",
//	            apiErr.Number, apiErr.Message, apiErr.Command)
//	    }
//	}
//
// Branch on a documented category with errors.Is against a sentinel:
//
//	if errors.Is(err, namecheap.ErrDomainNotFound) {
//	    // the domain is gone: recreate it
//	}
//
// Sentinels match through errors.Join, so errors.Is finds each code in a
// multi-error response. Decide whether to retry with IsRetryable, which treats
// transient server-side codes and transport timeouts as retryable while
// classifying validation, not-found, auth, context-cancellation and
// permission failures as permanent:
//
//	if namecheap.IsRetryable(err) {
//	    // back off and try again
//	}
//
// Distinguish a decode failure with errors.As on *ParseError; it keeps a
// bounded snippet of the raw body for diagnostics:
//
//	var parseErr *namecheap.ParseError
//	if errors.As(err, &parseErr) {
//	    log.Printf("bad response: %s", parseErr.Snippet)
//	}
package namecheap
