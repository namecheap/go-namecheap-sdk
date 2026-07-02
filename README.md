# Go Namecheap SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/namecheap/go-namecheap-sdk.svg)](https://pkg.go.dev/github.com/namecheap/go-namecheap-sdk/v2)

- [Namecheap API Documentation](https://www.namecheap.com/support/api/intro/)
- [Sandbox](https://www.namecheap.com/support/knowledgebase/article.aspx/763/63/what-is-sandbox/)

### Getting

```sh
$ go get github.com/namecheap/go-namecheap-sdk/v2
```

### Usage

```go
import (
    "context"

    "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
)

client := namecheap.NewClient(&namecheap.ClientOptions{
    UserName:   "UserName",
    ApiUser:    "ApiUser",
    ApiKey:     "ApiKey",
    ClientIp:   "10.10.10.10",
    UseSandbox: false,
})

// Every call takes a context.Context as its first argument. Cancelling the
// context aborts the in-flight HTTP request, a pending rate-limit or
// concurrency wait, and any inter-retry backoff sleep.
ctx := context.Background()
```

> **Note:** The context-less methods (`Domains.GetInfo`, `DomainsDNS.SetHosts`,
> `DomainsNS.Create`, `Client.DoXML`, etc.) are deprecated. They delegate to
> their `...WithContext` counterparts with `context.Background()` and will be
> removed in v3. Prefer the `...WithContext` variants shown below.

### Available methods

#### Domains (`client.Domains`)

| Method | Description |
|---|---|
| `GetListWithContext(ctx, args)` | List domains for the account |
| `GetInfoWithContext(ctx, domain)` | Get detailed info about a domain |
| `CheckWithContext(ctx, domains...)` | Check availability of one or more domains |
| `GetTldListWithContext(ctx)` | List all TLDs and their per-TLD API capabilities |
| `CreateWithContext(ctx, args)` | Register a new domain (charge-bearing) |
| `RenewWithContext(ctx, args)` | Renew an expiring domain (charge-bearing) |
| `ReactivateWithContext(ctx, args)` | Reactivate an expired domain (charge-bearing) |
| `GetContactsWithContext(ctx, domain)` | Get a domain's contact information |
| `SetContactsWithContext(ctx, args)` | Set a domain's contact information |
| `GetRegistrarLockWithContext(ctx, domain)` | Get the registrar-lock status |
| `SetRegistrarLockWithContext(ctx, domain, action)` | Lock/unlock the domain at the registrar |

```go
// Check domain availability
resp, err := client.Domains.CheckWithContext(ctx, "example.com", "example.net")
if err != nil {
    log.Fatal(err)
}
for _, result := range *resp.DomainCheckResults {
    fmt.Printf("%s available=%v\n", *result.Domain, *result.IsAvailable)
}
```

```go
// Register a new domain. The four contact blocks reuse the shared
// namecheap.ContactInfo type; all required fields are validated up front and
// every missing field is reported at once as a *namecheap.InvalidArgumentsError.
contact := namecheap.ContactInfo{
    FirstName:    "John",
    LastName:     "Smith",
    Address1:     "8939 S.cross Blvd",
    City:         "Phoenix",
    StateProvince: "AZ",
    PostalCode:   "85284",
    Country:      "US",
    Phone:        "+1.6613102107",
    EmailAddress: "john@example.com",
}
created, err := client.Domains.CreateWithContext(ctx, &namecheap.DomainsCreateArgs{
    DomainName: "example.com",
    Years:      2,
    Registrant: contact,
    Tech:       contact,
    Admin:      contact,
    AuxBilling: contact,
    // For a premium domain set IsPremiumDomain and PremiumPrice together; the
    // premium guard rejects the call (before any charge) if they disagree.
    // IsPremiumDomain: true,
    // PremiumPrice:    namecheap.Amount("13000.0000"),
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("registered=%v charged=%s\n",
    *created.DomainCreateResult.Registered, created.DomainCreateResult.ChargedAmount)

// Renew an expiring domain.
renewed, err := client.Domains.RenewWithContext(ctx, &namecheap.DomainsRenewArgs{
    DomainName: "example.com",
    Years:      1,
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("charged=%s\n", renewed.DomainRenewResult.ChargedAmount)
```

> **Money and charge-bearing calls.** `Create`, `Renew` and `Reactivate` can
> charge the account. Their prices are exposed as `namecheap.Amount` — a string
> type that preserves the exact server value (money is **never** a `float64`, to
> avoid decimal rounding). These three calls are treated as **non-idempotent**:
> on an ambiguous transport or server-side failure the SDK does **not** retry
> (a resend could double-charge); only Namecheap's pre-execution HTTP 405
> rate-limit signal is retried. Reconcile such failures via the account order
> history. All other methods remain idempotent and retry as before.

#### Domains API coverage matrix

| `namecheap.domains.*` command | Status |
|---|---|
| `getList` | Implemented |
| `getInfo` | Implemented |
| `check` | Implemented |
| `getTldList` | Implemented |
| `create` | Implemented |
| `renew` | Implemented |
| `reactivate` | Implemented |
| `getContacts` | Implemented |
| `setContacts` | Implemented |
| `getRegistrarLock` | Implemented |
| `setRegistrarLock` | Implemented |
| `getRegistrarLockStatus` (bulk) | Planned |
| `transfer.*` | Planned |

#### DomainsDNS (`client.DomainsDNS`)

| Method | Description |
|---|---|
| `GetListWithContext(ctx, domain)` | Get the nameservers for a domain |
| `SetDefaultWithContext(ctx, domain)` | Switch domain to Namecheap default DNS |
| `SetCustomWithContext(ctx, domain, nameservers)` | Switch domain to custom nameservers |
| `GetHostsWithContext(ctx, domain)` | Get DNS host records |
| `SetHostsWithContext(ctx, args)` | Set DNS host records |
| `GetEmailForwardingWithContext(ctx, domain)` | Get email forwarding rules |
| `SetEmailForwardingWithContext(ctx, domain, forwards)` | Set email forwarding rules |

```go
// Set DNS host records
resp, err := client.DomainsDNS.SetHostsWithContext(ctx, &namecheap.DomainsDNSSetHostsArgs{
    Domain: namecheap.String("domain.com"),
    Records: &[]namecheap.DomainsDNSHostRecord{
        {
            HostName:   namecheap.String("blog"),
            RecordType: namecheap.String("A"),
            Address:    namecheap.String("11.12.13.14"),
        },
    },
})

// Get DNS host records
resp, err := client.DomainsDNS.GetHostsWithContext(ctx, "domain.com")

// Manage email forwarding
forwards, err := client.DomainsDNS.GetEmailForwardingWithContext(ctx, "domain.com")

_, err = client.DomainsDNS.SetEmailForwardingWithContext(ctx, "domain.com", []namecheap.EmailForward{
    {Mailbox: "info", ForwardTo: "user@example.com"},
})
```

#### DomainsNS (`client.DomainsNS`)

| Method | Description |
|---|---|
| `CreateWithContext(ctx, sld, tld, nameserver, ip)` | Register a new nameserver |
| `GetInfoWithContext(ctx, sld, tld, nameserver)` | Get info about a registered nameserver |
| `UpdateWithContext(ctx, sld, tld, nameserver, oldIP, ip)` | Update a nameserver IP address |
| `DeleteWithContext(ctx, sld, tld, nameserver)` | Delete a registered nameserver |

```go
// Create a custom nameserver
_, err := client.DomainsNS.CreateWithContext(ctx, "domain", "com", "ns1.domain.com", "1.2.3.4")

// Delete a nameserver
_, err = client.DomainsNS.DeleteWithContext(ctx, "domain", "com", "ns1.domain.com")
```

### Error handling

When the API rejects a call it returns a typed `*namecheap.APIError` carrying the
numeric code, the server message and the failing command. The error string keeps
the legacy `"<message> (<number>)"` format, so code that matches on it today keeps
working:

```go
resp, err := client.Domains.GetInfoWithContext(ctx, "example.com")
if err != nil {
    log.Fatal(err) // e.g. "Domain not found (2019166)"
}
```

Inspect the structured code with `errors.As`:

```go
var apiErr *namecheap.APIError
if errors.As(err, &apiErr) {
    log.Printf("code=%d message=%q command=%q", apiErr.Number, apiErr.Message, apiErr.Command)
}
```

Branch on a documented category with `errors.Is` against a sentinel (matches
through `errors.Join` for multi-error responses):

```go
if errors.Is(err, namecheap.ErrDomainNotFound) {
    // the domain is gone: recreate it
}
```

Available sentinels: `ErrDomainNotFound`, `ErrDomainNotAssociated`,
`ErrDomainInvalid`, `ErrTooManyDomains`, `ErrPromotionCodeInvalid`,
`ErrOrderNotFound`, `ErrAccessDenied`, `ErrServerError`.

Decide whether to retry with `namecheap.IsRetryable`, which treats transient
server-side codes and transport timeouts as retryable and classifies validation,
not-found, auth, permission and context-cancellation failures as permanent:

```go
if namecheap.IsRetryable(err) {
    // back off and try again
}
```

Malformed responses return a `*namecheap.ParseError` (with a bounded snippet of
the raw body); transport and context errors propagate unwrapped. All error types
support `errors.Is` / `errors.As` for inspecting the underlying cause.

Client-side validation failures (missing required arguments, missing required
contact fields, or a tripped premium guard) are returned as a
`*namecheap.InvalidArgumentsError` **before** any request is sent. It lists every
offending field at once via its `Fields` slice, so a caller can fix them in a
single pass:

```go
var argErr *namecheap.InvalidArgumentsError
if errors.As(err, &argErr) {
    log.Printf("fix these fields: %v", argErr.Fields)
}
```

### Rate limiting & retries

The client is concurrency-safe and paces itself against Namecheap's published API
quotas. Every request flows through a client-side token-bucket rate limiter, an
optional in-flight concurrency bound, and a context-aware exponential-backoff
retry policy. All of it is configurable and falls back to safe defaults.

> **Behavior change:** requests are now **concurrent by default**. Earlier
> versions funneled every call through a single process-wide lock, so calls were
> strictly serialized. If you relied on that serialization, set
> `RateLimit.MaxConcurrent: 1` (or a low `RateLimit.PerMinute`) to restore it.

**Quotas.** Namecheap documents 20 requests/minute, 700/hour and 8000/day. The
limiter enforces the per-minute bucket (default 20/min, with a burst equal to the
per-minute rate so genuine concurrency under the quota is not throttled). The
hour and day budgets are *not* enforced client-side, to avoid stalling
long-running processes.

**Retries.** Only errors classified as retryable by `IsRetryable` (transient
server-side codes and transport timeouts) plus Namecheap's HTTP 405 rate-limit
signal are retried, using exponential backoff with equal jitter, bounded by a
per-attempt cap and a total wall-time budget. A context deadline or cancellation
aborts a limiter wait or a backoff sleep promptly. A terminal failure wraps the
last real error as `after N attempts: <cause>`, so `errors.Is`/`errors.As` still
reach the underlying `*APIError`.

```go
client := namecheap.NewClient(&namecheap.ClientOptions{
    UserName: "UserName",
    ApiUser:  "ApiUser",
    ApiKey:   "ApiKey",
    ClientIp: "10.10.10.10",

    // Identify your integration to Namecheap support (appended to the SDK's
    // default User-Agent on every request).
    UserAgent: "my-app/1.2.3",

    // Inject your own HTTP client or middleware (tracing, mocking, custom
    // timeouts). Transport is applied onto the effective client.
    // HTTPClient: myHTTPClient,
    // Transport:  myRoundTripper,

    RateLimit: &namecheap.RateLimitOptions{
        PerMinute:     20,    // token-bucket rate; default 20
        MaxConcurrent: 0,     // 0 = unbounded; 1 restores serial behavior
        Disabled:      false, // true = no client-side limiting
    },
    Retry: &namecheap.RetryOptions{
        MaxAttempts: 4,                     // total attempts incl. the first
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        MaxElapsed:  2 * time.Minute,       // cap on total retry wall-time
    },
})
```

Every resilience field is optional; a nil/zero value selects the default shown
above (`PerMinute: 20`, `MaxAttempts: 4`, `BaseDelay: 500ms`, `MaxDelay: 30s`,
`MaxElapsed: 2m`, unbounded concurrency). Pass
`RateLimit: &namecheap.RateLimitOptions{Disabled: true}` to turn off client-side
limiting entirely.

### Sandbox

Before you start using our API, we advise you to try it in our [Sandbox](https://www.sandbox.namecheap.com/) environment. The sandbox environment was created
explicitly for testing purposes. All purchases processed through the sandbox API are simulated.

To start testing API in Sandbox, you will need to sign up for an account here (this account will not be associated with
the one you have at http://www.namecheap.com).

```go
client := namecheap.NewClient(&namecheap.ClientOptions{
    // ...
    UseSandbox: true,
})
```

### Contributing

To contribute, please read our [contributing](CONTRIBUTING.md) docs.
