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

#### DomainsTransfer (`client.DomainsTransfer`)

| Method | Description |
|---|---|
| `CreateWithContext(ctx, args)` | Start an inbound domain transfer (charge-bearing) |
| `GetStatusWithContext(ctx, transferID)` | Get the status of a single transfer |
| `UpdateStatusWithContext(ctx, args)` | Resubmit a transfer after releasing the registry lock |
| `GetListWithContext(ctx, args)` | List transfers (filtered and paged) |
| `WaitForCompletionWithContext(ctx, transferID, opts...)` | Poll GetStatus until the transfer is terminal |

```go
// Start an inbound transfer, then wait for it to finish. EPPCode is a transfer
// authorization credential: it is redacted to "***" on every observability
// surface (request/response hooks and slog), exactly like ApiKey.
created, err := client.DomainsTransfer.CreateWithContext(ctx, &namecheap.DomainsTransferCreateArgs{
    DomainName: "example.com",
    Years:      1,
    EPPCode:    "the-auth-code", // redacted in hooks/logs
})
if err != nil {
    log.Fatal(err)
}
transferID := *created.DomainTransferCreateResult.TransferID
fmt.Printf("transfer %d charged=%s\n", transferID, created.DomainTransferCreateResult.ChargedAmount)

// Poll until the transfer reaches a terminal state (default interval 30s).
final, err := client.DomainsTransfer.WaitForCompletionWithContext(ctx, transferID,
    namecheap.WithPollInterval(60*time.Second))
if err != nil {
    log.Fatal(err) // includes a prompt return on ctx cancellation
}
fmt.Printf("final state=%s status=%q\n",
    final.TransferState(), *final.DomainTransferGetStatusResult.Status)
```

> **Transfer status classification.** The Namecheap API doc does not enumerate
> the numeric `StatusID` codes. This SDK exposes the raw `StatusID` (int) and the
> free-text `Status` verbatim on every response, and offers a small typed
> `TransferState` (`INPROGRESS` / `COMPLETED` / `CANCELLED` / `UNKNOWN`) grounded
> in the documented `getList` category vocabulary. `ClassifyTransferStatus`,
> `TransferState.IsTerminal()` and `TransferState.IsActionRequired()` classify a
> description by case-insensitive keyword matching — no fabricated code table.
>
> **`Create` is charge-bearing and non-idempotent** — same treatment as
> `domains.create`: never auto-retried on an ambiguous transport/server failure.
> `GetStatus`, `UpdateStatus` and `GetList` are idempotent and retry as usual.

#### DomainsTransfer API coverage matrix

| `namecheap.domains.transfer.*` command | Status |
|---|---|
| `create` | Implemented |
| `getStatus` | Implemented |
| `updateStatus` | Implemented |
| `getList` | Implemented |

#### DomainsDNS (`client.DomainsDNS`)

| Method | Description |
|---|---|
| `GetListWithContext(ctx, domain)` | Get the nameservers for a domain |
| `SetDefaultWithContext(ctx, domain)` | Switch domain to Namecheap default DNS |
| `SetCustomWithContext(ctx, domain, nameservers)` | Switch domain to custom nameservers |
| `GetHostsWithContext(ctx, domain)` | Get DNS host records |
| `SetHostsWithContext(ctx, args)` | Set (replace) the entire DNS host record set |
| `AddRecordsWithContext(ctx, domain, records, opts...)` | Add records, preserving all existing ones |
| `DeleteRecordsWithContext(ctx, domain, selector, opts...)` | Delete records matching a selector, preserving the rest |
| `UpsertRecordsWithContext(ctx, domain, selector, records, opts...)` | Replace the selector-matched records with new ones |
| `PlanWithContext(ctx, domain, ops...)` | Compute an add/remove/keep diff without writing |
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

#### Managing individual DNS records

`SetHosts` (`namecheap.domains.dns.setHosts`) is the only write endpoint the API
offers, and it **replaces the entire record set**. To change one record you must
read every record, edit the slice, and write them all back — forget one and it is
silently deleted (this is the [#49](https://github.com/namecheap/go-namecheap-sdk/issues/49)
footgun). The record-level helpers own that read-modify-write once, correctly.

Delete one record — the #49 question, answered in five lines:

```go
_, err := client.DomainsDNS.DeleteRecordsWithContext(ctx, "example.com",
    namecheap.RecordSelector{
        HostName:   namecheap.String("blog"),
        RecordType: namecheap.String("A"),
    })
// every other record (and the zone EmailType) is preserved automatically.
```

```go
// Add records, keeping everything else:
_, err := client.DomainsDNS.AddRecordsWithContext(ctx, "example.com",
    []namecheap.DomainsDNSHostRecord{
        {HostName: namecheap.String("www"), RecordType: namecheap.String("A"), Address: namecheap.String("1.2.3.4")},
    })

// Replace exactly the selector-matched records (upsert):
_, err = client.DomainsDNS.UpsertRecordsWithContext(ctx, "example.com",
    namecheap.RecordSelector{RecordType: namecheap.String("TXT")},
    []namecheap.DomainsDNSHostRecord{
        {HostName: namecheap.String("@"), RecordType: namecheap.String("TXT"), Address: namecheap.String("v=spf1 -all")},
    })

// Preview a change without writing (zero setHosts calls):
diff, err := client.DomainsDNS.PlanWithContext(ctx, "example.com",
    namecheap.DeleteOp(namecheap.RecordSelector{HostName: namecheap.String("old")}))
fmt.Println(diff) // RecordDiff: +0 -1 =7
```

A `RecordSelector` matches a record when **every** non-nil field equals the
record's (HostName/RecordType compared case-insensitively, a trailing dot on the
address ignored, MXPref exact). An empty selector is **rejected** with a typed
`*InvalidArgumentsError` to refuse an accidental mass-delete; set
`RecordSelector{MatchAll: true}` for an intentional full wipe.

> **⚠️ The API is not transactional.** `setHosts` replaces the whole zone, so a
> concurrent writer between your read and your write causes a lost update. Every
> mutating helper re-reads the zone after writing and, if the result does not
> match what it intended, returns `namecheap.ErrConcurrentModification`
> (`errors.Is`-matchable) instead of silently accepting the race:
>
> ```go
> _, err := client.DomainsDNS.AddRecordsWithContext(ctx, "example.com", records)
> if errors.Is(err, namecheap.ErrConcurrentModification) {
>     // someone else changed the zone; re-read and retry
> }
> ```
>
> Pass `namecheap.WithRetryOnConflict(n)` to have the helper retry the whole
> read-modify-write-verify cycle up to `n` times automatically:
>
> ```go
> _, err := client.DomainsDNS.AddRecordsWithContext(ctx, "example.com", records,
>     namecheap.WithRetryOnConflict(3))
> ```
>
> This is detection-plus-retry, not true transactionality (the API cannot offer
> that) — but it turns a silent data-loss footgun into an explicit, handleable
> error.

`NormalizeRecord` and `RecordsEqual` are exported so consumers can reuse the same
comparison logic (TTL defaults to 1799, hostname lower-cased, `@` apex handling,
trailing-dot handling, record type upper-cased).

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

#### Users (`client.Users`)

| Method | Description |
|---|---|
| `GetPricingWithContext(ctx, args)` | Get the full price sheet for a product type (DOMAIN/SSLCERTIFICATE/WHOISGUARD) |
| `GetBalancesWithContext(ctx)` | Get account funds (decimal-safe amounts + currency) |
| `CreateAddFundsRequestWithContext(ctx, args)` | Create a credit-card add-funds request (charge-bearing, non-idempotent) |
| `GetAddFundsStatusWithContext(ctx, tokenID)` | Get the status of an add-funds request |
| `ChangePasswordWithContext(ctx, args)` | Change the account password (old-password or reset-code method) |
| `UpdateWithContext(ctx, args)` | Update the account contact information |

The pricing response is a deeply nested tree (`ProductType → ProductCategory →
Product → Price` tiers). Navigate it directly, or use `PriceFor` for the common
single-tier lookup. Money is never parsed to `float64`: every amount is an
`Amount` (the exact server decimal string). `Price.EffectivePrice()` resolves the
documented precedence — server-resolved `Price` (which already reflects any
promo/special), then `YourPrice` (account price), then `RegularPrice` (list
price). The sheet is large and slow-changing, so fetch it once and cache it.

```go
// Example: check the balance before a bulk renew.
// Amount is an exact decimal string, never a float64 — convert it to a decimal
// type (here math/big.Rat) at the point you need the numeric comparison.
balances, err := client.Users.GetBalancesWithContext(ctx)
if err != nil {
    log.Fatal(err)
}
result := balances.UserGetBalancesResult
have, ok := new(big.Rat).SetString(result.AvailableBalance.String())
need := big.NewRat(10000, 100) // 100.00
if !ok || have.Cmp(need) < 0 {
    log.Fatalf("insufficient funds: %s %s available, gate the batch",
        result.Currency, result.AvailableBalance)
}
// ... proceed with the renew batch.
```

```go
// Example: find the cheapest TLD among com/net/org for a 1-year registration.
pricing, err := client.Users.GetPricingWithContext(ctx, &namecheap.UsersGetPricingArgs{
    ProductType: namecheap.String("DOMAIN"),
    ActionName:  namecheap.String("REGISTER"),
})
if err != nil {
    log.Fatal(err)
}
result := pricing.UserGetPricingResult
var cheapestTLD string
var cheapest *big.Rat
for _, tld := range []string{"com", "net", "org"} {
    price, ok := result.PriceFor("REGISTER", tld, 1)
    if !ok {
        continue
    }
    // Compare as decimals, not strings: "8.88" < "10.50" numerically, but not
    // lexicographically.
    eff, ok := new(big.Rat).SetString(price.EffectivePrice().String())
    if !ok {
        continue
    }
    if cheapest == nil || eff.Cmp(cheapest) < 0 {
        cheapestTLD, cheapest = tld, eff
    }
}
fmt.Printf("cheapest: .%s at %s\n", cheapestTLD, cheapest.FloatString(2))
```

> **Note:** `AddFundsRequest` is charge-bearing and **non-idempotent** — an
> ambiguous transport/server failure is never retried (only Namecheap's
> pre-execution HTTP 405 rate-limit signal is), so it can never double-charge.
> Reconcile an ambiguous failure with `GetAddFundsStatusWithContext`. The
> `changePassword` password values are only ever placed in the outbound request
> parameters — never stored, logged, or echoed in errors; hook-level redaction
> lands with the logging layer in [#113](https://github.com/namecheap/go-namecheap-sdk/issues/113).

##### Users API coverage matrix

| `namecheap.users.*` command | Status |
|---|---|
| `getPricing` | Implemented |
| `getBalances` | Implemented |
| `createaddfundsrequest` | Implemented |
| `getAddFundsStatus` | Implemented |
| `changePassword` | Implemented |
| `update` | Implemented |
| `create` | Planned, unscheduled (reseller-only account creation; weak demand) |
| `login` | Planned, unscheduled (validates only accounts made via `users.create`) |
| `resetPassword` | Planned, unscheduled (reseller account-creation flow) |

#### UsersAddress (`client.UsersAddress`)

The address book stores reusable registrant profiles. An entry holds the same
logical fields as a domain `ContactInfo`, so a stored address can feed the
contact blocks of `domains.create`.

| Method | Description |
|---|---|
| `CreateWithContext(ctx, details)` | Add a new address-book entry |
| `UpdateWithContext(ctx, addressID, details)` | Update an existing entry |
| `DeleteWithContext(ctx, addressID)` | Delete an entry |
| `GetInfoWithContext(ctx, addressID)` | Get the full stored address |
| `GetListWithContext(ctx)` | List every entry (id + name) |
| `SetDefaultWithContext(ctx, addressID)` | Mark an entry as the account default |

```go
// Reuse a stored address as a domains.create contact block.
info, err := client.UsersAddress.GetInfoWithContext(ctx, 777)
if err != nil {
    log.Fatal(err)
}
contact := info.GetAddressInfoResult.ToContactInfo()
_, err = client.Domains.CreateWithContext(ctx, &namecheap.DomainsCreateArgs{
    DomainName: "example.com",
    Years:      1,
    Registrant: contact, Tech: contact, Admin: contact, AuxBilling: contact,
})
```

The `ContactInfo` ↔ address-book adapter maps the twelve shared logical fields in
both directions (`ContactInfo.ToAddressDetails`, `UsersAddressDetails.ToContactInfo`,
`UsersAddressGetInfoResult.ToContactInfo`). Two field names differ between the two
API shapes and are mapped explicitly: `PostalCode` ↔ `Zip` and `OrganizationName`
↔ `Organization`. The address book also carries five fields `ContactInfo` has no
counterpart for (`AddressName`, `DefaultYN`, `StateProvinceChoice`, `PhoneExt`,
`Fax`); set `StateProvinceChoice` yourself before create/update, as it is required.

##### UsersAddress API coverage matrix

| `namecheap.users.address.*` command | Status |
|---|---|
| `create` | Implemented |
| `update` | Implemented |
| `delete` | Implemented |
| `getInfo` | Implemented |
| `getList` | Implemented |
| `setDefault` | Implemented |

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

### Observability

The SDK exposes safe, first-class hooks into the request pipeline so you can log,
trace and measure calls in production without ever wrapping the raw HTTP client
(which would see the credential — `ApiKey` travels as a form field). Everything
is opt-in: with no hooks and no logger configured, the observability path does no
work and allocates nothing.

#### Redaction guarantee

`RequestInfo.Params` is always a **redacted copy**: the value of every secret
parameter is replaced with `***` before it ever reaches a hook or a log record.
The redacted key set is **`ApiKey`, `NewPassword`, `OldPassword`, `ResetCode`,
`EPPCode`** and is trivial to extend in one place. The SDK never hands a live parameter map
to a hook and never logs an unredacted parameter — redaction is enforced by
construction, not by convention.

#### Ordering

Per attempt (retries included), the pipeline is:

1. rate-limiter wait (a `Debug` slog event records the wait), then
2. `OnRequest` / slog request-start fires **immediately before** the HTTP send, then
3. the HTTP round trip, then
4. `OnResponse` / slog request-end fires after the attempt completes, carrying
   the status, duration, error code and whether a retry will follow.

The rate-limiter wait therefore happens **before** `OnRequest`, so a hook's
timestamp reflects the moment the request is actually sent, not when it was
queued.

#### Hooks

Both hooks are optional and fire once per attempt with a 1-based `Attempt`. A
panicking hook is recovered (and logged if a `Logger` is set); it never crashes
the caller or aborts the request.

```go
client := namecheap.NewClient(&namecheap.ClientOptions{
    UserName: "UserName", ApiUser: "ApiUser", ApiKey: "ApiKey", ClientIp: "10.10.10.10",

    OnRequest: func(info namecheap.RequestInfo) {
        // info.Params is already redacted; info.Attempt is 1-based.
        log.Printf("→ %s attempt=%d params=%v", info.Command, info.Attempt, info.Params)
    },
    OnResponse: func(info namecheap.ResponseInfo) {
        log.Printf("← %s attempt=%d status=%d dur=%s code=%d willRetry=%t err=%v",
            info.Command, info.Attempt, info.StatusCode, info.Duration,
            info.ErrorCode, info.WillRetry, info.Err)
    },
})
```

#### Structured logging (`log/slog`)

Set `Logger` to emit structured events on the same path — no extra dependency
(stdlib `log/slog`). Levels are chosen so steady state is quiet: request start
and limiter wait at `Debug`, success at `Info`, retryable failure/retry at
`Warn`. Records carry `command`, `attempt`, `duration`, `status` and
`error_code` (and `retry_delay`/`retry_reason` on a retry). Logged parameters are
the redacted copy. This maps cleanly onto the Terraform provider's `TF_LOG`
bridge.

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
client := namecheap.NewClient(&namecheap.ClientOptions{
    UserName: "UserName", ApiUser: "ApiUser", ApiKey: "ApiKey", ClientIp: "10.10.10.10",
    Logger: logger,
})
```

#### Stats

`Client.Stats()` returns a snapshot (a deep copy — mutating it never affects the
client) suitable for exporting to Prometheus/OTel in a few lines, without the SDK
depending on either:

```go
s := client.Stats()
// s.RequestsByCommand map[string]int64   – calls per command
// s.ErrorsByCode      map[int]int64       – failed attempts by Namecheap code
// s.Retries           int64               – retry attempts (beyond the first)
// s.TotalLimiterWait  time.Duration       – cumulative rate-limiter wait
// s.QuotaRemaining    int                 – best-effort minute-bucket estimate
```

`QuotaRemaining` is an estimate of the tokens currently left in the limiter's
minute bucket (0 when rate limiting is disabled); treat it as a hint, not a hard
count.

#### OpenTelemetry (`otelnamecheap`)

Tracing lives in a **separate, opt-in module** (`otelnamecheap`) with its own
`go.mod`, so the core SDK stays dependency-free. It provides a RoundTripper you
wire through `ClientOptions.Transport`; each API call produces a client span per
HTTP attempt with the command and HTTP status as attributes, and an error status
for non-2xx responses or transport failures. It reads the request body only to
extract the command name — never a secret.

```go
import (
    "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
    "github.com/namecheap/go-namecheap-sdk/v2/otelnamecheap"
)

client := namecheap.NewClient(&namecheap.ClientOptions{
    UserName: "UserName", ApiUser: "ApiUser", ApiKey: "ApiKey", ClientIp: "10.10.10.10",
    // nil base wraps http.DefaultTransport; the global TracerProvider is used
    // unless you pass otelnamecheap.WithTracerProvider(tp).
    Transport: otelnamecheap.NewTransport(nil),
})
```

Add it to your project with:

```sh
go get github.com/namecheap/go-namecheap-sdk/v2/otelnamecheap
```

#### Why no wire-level HTTP dumps

The SDK deliberately does **not** offer a raw request/response body dump. The
credential travels in the POST body, and a byte-level dump cannot guarantee
redaction — so the safe hooks above are the supported surface. If you truly need
low-level tracing you can attach `net/http/httptrace` to your context, **but be
warned**: httptrace exposes the unredacted request and can leak `ApiKey` and
passwords into your logs. That path is out of scope for the SDK's redaction
guarantee and is your responsibility to handle safely.

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
