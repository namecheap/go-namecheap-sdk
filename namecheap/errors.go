package namecheap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// maxSnippetBytes caps how many bytes of a malformed response body a ParseError
// retains, so a huge or hostile body can never pin unbounded memory in the
// error chain.
const maxSnippetBytes = 512

// APIError is a typed, machine-matchable representation of a single
// <Error Number="..."> entry returned by the Namecheap API when a call fails
// (envelope Status="ERROR").
//
// Its Error string is intentionally formatted as "<message> (<number>)" to
// preserve the exact text the SDK produced before typed errors existed, so
// consumers that string-match today keep working.
//
// Match against it with errors.As to read the code programmatically:
//
//	var apiErr *namecheap.APIError
//	if errors.As(err, &apiErr) {
//	    log.Printf("code=%d msg=%q command=%q", apiErr.Number, apiErr.Message, apiErr.Command)
//	}
//
// or against a sentinel with errors.Is:
//
//	if errors.Is(err, namecheap.ErrDomainNotFound) { /* recreate resource */ }
type APIError struct {
	Number  int    // Namecheap numeric error code, e.g. 2019166.
	Message string // Server-provided human-readable message.
	Command string // Command that failed, e.g. "namecheap.domains.getInfo".
}

// Error implements the error interface. The format is preserved verbatim from
// the pre-typed-errors SDK: "<message> (<number>)".
func (e *APIError) Error() string {
	return fmt.Sprintf("%s (%d)", e.Message, e.Number)
}

// Is reports whether target matches this API error. It returns true when target
// is a sentinel whose documented code set contains e.Number, and when target is
// another *APIError carrying the same Number. This makes both
// errors.Is(err, namecheap.ErrDomainNotFound) and
// errors.Is(err, &namecheap.APIError{Number: 2019166}) work naturally,
// including through errors.Join and wrapping.
func (e *APIError) Is(target error) bool {
	switch t := target.(type) {
	case *sentinelError:
		for _, n := range t.numbers {
			if n == e.Number {
				return true
			}
		}
		return false
	case *APIError:
		return t.Number == e.Number
	default:
		return false
	}
}

// sentinelError is an unexported category error carrying a name and the set of
// Namecheap numeric codes it stands for. Instances are exported as Err* values
// and are matched by (*APIError).Is, so errors.Is(apiErr, ErrX) succeeds when
// apiErr.Number is in the set.
type sentinelError struct {
	name    string
	numbers []int
}

// Error implements the error interface for use as a sentinel.
func (e *sentinelError) Error() string { return e.name }

// serverErrorNumbers is the set of documented codes that represent transient,
// server-side failures worth retrying. It is the single source of truth shared
// by ErrServerError and IsRetryable.
//
// docs/namecheap-api-v2.md § Error Codes.
var serverErrorNumbers = []int{3050900, 5019169, 5050169, 5050900}

// Sentinel errors for the documented, actionable Namecheap error codes.
//
// Every code below is verified present in docs/namecheap-api-v2.md; no codes are
// invented. Match with errors.Is. The raw APIError.Number covers the long tail
// of codes that do not have a dedicated sentinel.
//
// Code -> sentinel table:
//
//	2019166                     -> ErrDomainNotFound
//	2016166, 3016166            -> ErrDomainNotAssociated
//	2030166                     -> ErrDomainInvalid
//	2011169                     -> ErrTooManyDomains
//	2011170                     -> ErrPromotionCodeInvalid
//	2033409                     -> ErrOrderNotFound
//	4011103                     -> ErrAccessDenied
//	3050900, 5019169,
//	5050169, 5050900            -> ErrServerError
//
// Deliberate, documented gap: the issue also names ErrRateLimited,
// ErrNotWhitelistedIP, ErrAuthFailure and ErrInsufficientFunds, but
// docs/namecheap-api-v2.md carries NO numeric codes for these categories, so no
// sentinels are fabricated for them. Rate limiting in particular is surfaced by
// the API as HTTP 405 and is already absorbed by the transport retry layer (see
// DoXMLWithContext / IsRetryable), not as a numeric <Error>. Add these
// sentinels only once real codes are confirmed and added to the doc.
var (
	// ErrDomainNotFound matches "Domain not found" / "Domain name not found".
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrDomainNotFound error = &sentinelError{name: "domain not found", numbers: []int{2019166}}

	// ErrDomainNotAssociated matches a domain not associated with the account
	// (2016166) or not associated with Enom (3016166).
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrDomainNotAssociated error = &sentinelError{name: "domain not associated with account", numbers: []int{2016166, 3016166}}

	// ErrDomainInvalid matches an invalid domain / unsupported edit permission.
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrDomainInvalid error = &sentinelError{name: "domain invalid", numbers: []int{2030166}}

	// ErrTooManyDomains matches passing more than 50 domains to a check call.
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrTooManyDomains error = &sentinelError{name: "too many domains", numbers: []int{2011169}}

	// ErrPromotionCodeInvalid matches an invalid promotion code.
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrPromotionCodeInvalid error = &sentinelError{name: "promotion code invalid", numbers: []int{2011170}}

	// ErrOrderNotFound matches an order that could not be found.
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrOrderNotFound error = &sentinelError{name: "order not found", numbers: []int{2033409}}

	// ErrAccessDenied matches an access-denied / permission error.
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrAccessDenied error = &sentinelError{name: "access denied", numbers: []int{4011103}}

	// ErrServerError matches unknown/unhandled transient server exceptions. It
	// shares its code set with IsRetryable.
	//
	// docs/namecheap-api-v2.md § Error Codes.
	ErrServerError error = &sentinelError{name: "server error", numbers: serverErrorNumbers}
)

// ErrConcurrentModification is returned by the record-level DNS helpers
// (AddRecordsWithContext, DeleteRecordsWithContext, UpsertRecordsWithContext)
// when the post-write verification read shows a record set that differs from
// the one the helper intended to write. Because namecheap.domains.dns.setHosts
// replaces the entire zone and is not transactional, this signals a lost-update
// race: another writer changed the zone between the helper's read and its write.
//
// It is a client-side sentinel (not an *APIError). Match it with errors.Is:
//
//	if errors.Is(err, namecheap.ErrConcurrentModification) { /* re-read and retry */ }
//
// Pass WithRetryOnConflict to have the helper retry the whole
// read-modify-write-verify cycle automatically instead of returning this error.
var ErrConcurrentModification = errors.New("concurrent modification detected: DNS zone changed between read and write")

// InvalidArgumentsError is a client-side validation error returned before any
// HTTP request is made, when method arguments are missing or inconsistent. It
// reports every offending field at once (not just the first) so a caller can fix
// them in a single pass.
//
// It is used for missing required contact fields (see ContactInfo) and for the
// premium-domain money-safety guard on charge-bearing calls. Match it with
// errors.As:
//
//	var argErr *namecheap.InvalidArgumentsError
//	if errors.As(err, &argErr) {
//	    log.Printf("invalid fields: %v", argErr.Fields)
//	}
type InvalidArgumentsError struct {
	// Fields lists the names of the invalid or missing arguments, using the
	// Namecheap parameter names (e.g. "RegistrantFirstName", "PremiumPrice").
	Fields []string
	// Reason, when set, explains why the fields are invalid. When empty the
	// fields are treated as plain "missing required" fields.
	Reason string
}

// Error implements the error interface, listing every offending field.
func (e *InvalidArgumentsError) Error() string {
	switch {
	case e.Reason != "" && len(e.Fields) > 0:
		return fmt.Sprintf("invalid arguments: %s (%s)", e.Reason, strings.Join(e.Fields, ", "))
	case e.Reason != "":
		return "invalid arguments: " + e.Reason
	default:
		return "missing required fields: " + strings.Join(e.Fields, ", ")
	}
}

// ParseError signals that a server response could not be decoded as the
// expected XML. It preserves a bounded snippet of the raw body (at most
// maxSnippetBytes) for diagnostics and wraps the underlying decode error, so
// consumers can tell an SDK/parsing fault apart from an API rejection
// (*APIError) or a transport/context failure.
//
// Its Error string keeps the "unable to parse server response:" prefix used by
// the pre-typed-errors SDK.
type ParseError struct {
	Snippet string // Bounded prefix of the raw response body.
	Err     error  // Underlying XML decode error.
}

// Error implements the error interface, preserving the legacy prefix.
func (e *ParseError) Error() string {
	return fmt.Sprintf("unable to parse server response: %v", e.Err)
}

// Unwrap exposes the underlying decode error for errors.Is/errors.As.
func (e *ParseError) Unwrap() error { return e.Err }

// IsRetryable reports whether err represents a transient failure that a caller
// may reasonably retry. It is the single source of truth for retryability so a
// retry policy (e.g. the resilience layer) does not have to re-derive it.
//
// It returns true for a transient server-side *APIError (serverErrorNumbers)
// and for a transport timeout (a net.Error with Timeout() == true). It returns
// false for context.Canceled and context.DeadlineExceeded (caller intent, even
// when wrapped in a *url.Error that satisfies net.Error), for any other
// *APIError (validation, not-found, auth and permission failures are
// permanent), and for everything else.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Caller intent must win over the net.Error check below, because a context
	// error can surface wrapped in a *url.Error whose Timeout() reports true.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return isServerErrorCode(apiErr.Number)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	return false
}

// isServerErrorCode reports whether n is a transient server-side error code.
func isServerErrorCode(n int) bool {
	for _, c := range serverErrorNumbers {
		if c == n {
			return true
		}
	}
	return false
}

// snippet returns at most maxSnippetBytes of data as a string.
func snippet(data []byte) string {
	if len(data) > maxSnippetBytes {
		return string(data[:maxSnippetBytes])
	}
	return string(data)
}

// atoiOrZero parses the numeric error code, defaulting to 0 when the pointer is
// nil or the value is not an integer.
func atoiOrZero(s *string) int {
	if s == nil {
		return 0
	}
	n, err := strconv.Atoi(*s)
	if err != nil {
		return 0
	}
	return n
}
