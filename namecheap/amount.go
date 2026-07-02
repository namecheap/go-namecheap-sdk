package namecheap

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
