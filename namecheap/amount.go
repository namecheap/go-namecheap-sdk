package namecheap

import "strings"

// Amount is a monetary value kept as the exact decimal string the Namecheap API
// returned (or that is sent to it), for example "10.87".
//
// Money is deliberately NOT modeled as float64. Binary floating point cannot
// represent most decimal fractions exactly, so round-tripping a price through a
// float can silently change it (10.87 becoming 10.869999999999999, and back to
// 10.87 only by luck of rounding). Keeping the raw server string preserves both
// the precise value and the server's own formatting, which matters for a
// charge-bearing API. Convert it to a decimal type of your choice (for example
// github.com/shopspring/decimal or math/big.Rat) at the point you actually need
// arithmetic, so the rounding policy is yours and explicit.
type Amount string

// String returns the raw amount string, e.g. "10.87". It is provided so an
// Amount can be used where a plain string is expected without an explicit
// conversion.
func (a Amount) String() string { return string(a) }

// IsPositive reports whether a is a present, positive money value: "8.88" and
// "0.01" are positive, while "", "   ", "0", "0.00" and "-1.00" are not.
//
// It is decimal-safe by construction — it never parses the amount to a float
// (see the type doc) — so it cannot mangle the value it is inspecting. It looks
// for a non-zero digit in an unsigned amount, which is exactly the question
// "did the server quote a real amount here", and no more: it does not validate
// the string as a number, so a malformed value the API is not documented to
// return (say "1,16") is reported by whether it contains a non-zero digit.
//
// It answers the question the getPricing attributes keep raising — the live API
// sends PromotionPrice="0.0" on tiers that carry no promotion, so presence of
// the attribute is not the same question as "is there a discount". Use this to
// ask the second one.
func (a Amount) IsPositive() bool {
	s := strings.TrimSpace(string(a))
	if strings.HasPrefix(s, "-") {
		return false
	}
	for _, r := range s {
		if r >= '1' && r <= '9' {
			return true
		}
	}
	return false
}
