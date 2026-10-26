package namecheap

import (
	"fmt"
	"strings"
	"time"
)

// DateTime is a date or timestamp as the Namecheap XML API renders it. The API
// has no single date format: each command formats its own fields, and the
// renew command's format even depends on the server that answered. The forms
// below are the ones observed on the live API, with the evidence behind each
// one (production and sandbox captures, October 2026):
//
//   - "11/19/2026" (zero-padded MM/DD/YYYY, date only): domains.getList
//     Created/Expires, domains.getInfo CreatedDate/ExpiredDate and Whoisguard
//     ExpiredDate, whoisguard.getList Created/Expires, ssl.getList
//     PurchaseDate. Captured on production.
//   - "1/20/2014" (non-padded M/D/YYYY): ssl.getInfo IssuedOn/Expires as shown
//     on the official docs page.
//   - "12/14/2034 1:01:07 PM": domains.renew DomainDetails/ExpiredDate as
//     captured from a real sandbox renew on 2026-10-26: date and time, the
//     hour not padded, and U+202F (narrow no-break space) before the AM/PM
//     designator. The official docs page shows the same field with an ASCII
//     space ("4/30/2021 11:31:13 AM").
//   - "4/30/21" and "4/30/21 11:31:13 AM" (two-digit year): reported by a
//     consumer on go-namecheap-sdk PR #177; not reproduced, accepted anyway.
//   - "2027-06-02T13:01:07" (ISO, no zone): PremiumDnsSubscription
//     CreatedDate/ExpirationDate, where "0001-01-01T00:00:00" means never.
//   - "" (empty element): ssl.getList ExpireDate/ActivationExpireDate on
//     certificates without one; decodes to the zero time.
//
// The API carries no zone information, so every value is taken as UTC wall
// clock; the server's GMTTimeDifference is not applied.
//
// An unrecognised value never fails decoding of the response that carries it
// (domains.renew is charge-bearing and deliberately never retried, so a date
// format drift must not turn a successful renewal into an SDK error): the text
// is kept verbatim in Raw, Time stays zero and ParseError reports the reason.
type DateTime struct {
	time.Time
	// Raw is the element text as received, trimmed. It is set for parsed and
	// unparsed values alike; for an unparsed value it is the only information
	// the API gave.
	Raw string

	err error
}

// dateTimeLayouts are tried in order against the normalised text. Order
// matters only for speed: the layouts are mutually exclusive on real input.
var dateTimeLayouts = []string{
	"1/2/2006",             // date only, padded or not (getList, getInfo, whoisguard, ssl)
	"1/2/2006 3:04:05 PM",  // renew ExpiredDate on an en-US host (sandbox capture, docs page)
	"1/2/2006 15:04:05",    // renew ExpiredDate on a 24-hour / invariant-culture host
	"1/2/06 3:04:05 PM",    // two-digit year with time (consumer report, PR #177)
	"1/2/06",               // two-digit year, date only (consumer report, PR #177)
	"2006-01-02T15:04:05",  // ISO without zone (PremiumDnsSubscription)
	"2006-01-02T15:04:05Z", // ISO with a Z designator, accepted for safety
}

// normaliseDateText maps the space characters the API may emit between the
// time and the AM/PM designator (U+202F narrow no-break space, U+00A0 no-break
// space) to a plain space and collapses runs of spaces, so one layout covers
// every variant.
func normaliseDateText(s string) string {
	s = strings.NewReplacer(" ", " ", " ", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func (dt DateTime) String() string {
	if dt.err != nil {
		return dt.Raw
	}
	return dt.Time.String()
}

// ParseError reports why the API text could not be parsed, or nil when it was
// (or was empty). Callers that render dates should fall back to Raw when it is
// non-nil instead of showing the zero time.
func (dt DateTime) ParseError() error {
	return dt.err
}

// UnmarshalText parses an API date in any of the forms listed on DateTime,
// ignoring surrounding whitespace. Empty or whitespace-only input yields the
// zero time without error: the API omits or empties date elements for objects
// that lack them. Unrecognised input also returns nil (see DateTime); the
// failure is available from ParseError.
func (dt *DateTime) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	dt.Raw = trimmed
	dt.err = nil
	dt.Time = time.Time{}
	if trimmed == "" {
		return nil
	}

	normalised := normaliseDateText(trimmed)
	var firstErr error
	for _, layout := range dateTimeLayouts {
		parsed, err := time.Parse(layout, normalised)
		if err == nil {
			dt.Time = parsed
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	dt.err = fmt.Errorf("unrecognised Namecheap date %q: %w", trimmed, firstErr)
	return nil
}

// Equal reports whether dt and u are equal based on time.Equal
func (dt DateTime) Equal(u DateTime) bool {
	return dt.Time.Equal(u.Time)
}
