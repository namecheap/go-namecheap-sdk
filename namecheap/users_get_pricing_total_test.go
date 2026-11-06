package namecheap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPrice_TotalPrice pins the pairing of each price attribute with its
// additional cost and the exact decimal addition.
func TestPrice_TotalPrice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		p         Price
		effective Amount
		want      Amount
	}{
		{"price plus its fee (the captured .com tier)", Price{Price: "13.98", AdditionalCost: "0.20", YourPrice: "13.98", YourAdditionalCost: "0.20", RegularPrice: "13.98", RegularAdditionalCost: "0.20"}, "13.98", "14.18"},
		{"fee pairs with the attribute that won", Price{Price: "0.0", AdditionalCost: "9.99", YourPrice: "10.50", YourAdditionalCost: "0.18", RegularPrice: "12.00", RegularAdditionalCost: "0.20"}, "10.50", "10.68"},
		{"regular price and its fee when nothing else is positive", Price{Price: "0.0", YourPrice: "0.0", RegularPrice: "12.00", RegularAdditionalCost: "0.20"}, "12.00", "12.20"},
		{"no fee attribute", Price{Price: "8.88"}, "8.88", "8.88"},
		{"zero fee", Price{Price: "8.88", AdditionalCost: "0.0"}, "8.88", "8.88"},
		{"scale follows the longer operand", Price{Price: "9", AdditionalCost: "0.185"}, "9", "9.185"},
		{"malformed fee leaves the price", Price{Price: "8.88", AdditionalCost: "1,16"}, "8.88", "8.88"},
		{"nothing positive returns RegularPrice unchanged", Price{Price: "0.0", YourPrice: "", RegularPrice: "0.00", RegularAdditionalCost: "0.20"}, "0.00", "0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.p.TotalPrice())
			assert.Equal(t, tc.effective, tc.p.EffectivePrice(), "EffectivePrice leaves the fee out")
		})
	}
}

func TestPrice_EffectivePriceIgnoresAdditionalCost(t *testing.T) {
	t.Parallel()
	p := Price{Price: "13.98", AdditionalCost: "0.20"}
	assert.Equal(t, Amount("13.98"), p.EffectivePrice())
	assert.Equal(t, Amount("14.18"), p.TotalPrice())
}
