package namecheap

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAmountString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "10.87", Amount("10.87").String())
	assert.Equal(t, "", Amount("").String())
}

// TestAmountIsPositive pins the money-presence predicate, including the two
// shapes the live API actually uses to mean "nothing here" ("" and "0.0") and
// the sign handling a bare digit scan would get wrong.
func TestAmountIsPositive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    Amount
		want bool
	}{
		{"typical price", "8.88", true},
		{"whole units", "10.00", true},
		{"smallest unit", "0.01", true},
		{"large amount", "1234567.89", true},
		{"leading zeros", "007.50", true},
		{"padded value", "  8.88  ", true},
		{"empty", "", false},
		{"whitespace only", "   ", false},
		{"bare zero", "0", false},
		{"zero decimal", "0.00", false},
		{"short zero — the live no-promotion sentinel", "0.0", false},
		{"many-decimal zero", "0.000000", false},
		{"negative", "-1.00", false},
		{"negative zero", "-0.00", false},
		{"padded negative", "  -12.34  ", false},
		// Not a validator: a malformed value the API is not documented to send is
		// judged on whether it carries a non-zero digit, not on being a number.
		{"comma decimal", "1,16", true},
		{"non-numeric", "N/A", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.a.IsPositive(), "Amount(%q).IsPositive()", tc.a)
		})
	}
}

// TestAmountXMLExactPreservation asserts an Amount decodes from an attribute
// verbatim, with no float rounding of a value that binary floats cannot hold.
func TestAmountXMLExactPreservation(t *testing.T) {
	t.Parallel()
	type holder struct {
		XMLName xml.Name `xml:"x"`
		Charged *Amount  `xml:"ChargedAmount,attr"`
	}
	var h holder
	err := xml.Unmarshal([]byte(`<x ChargedAmount="10.87"/>`), &h)
	assert.NoError(t, err)
	if assert.NotNil(t, h.Charged) {
		assert.Equal(t, Amount("10.87"), *h.Charged)
		assert.Equal(t, "10.87", h.Charged.String())
	}
}
