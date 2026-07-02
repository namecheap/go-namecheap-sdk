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
