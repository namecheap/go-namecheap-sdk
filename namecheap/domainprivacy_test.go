package namecheap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClassifyPrivacyStatus table-tests the status classifier over
// representative getList Status descriptions. The classifier is grounded in the
// documented getList ListType vocabulary (ALL | ALLOTED | FREE | DISCARD); the
// doc enumerates no Status values, so nothing here keys off a fabricated code
// table.
func TestClassifyPrivacyStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status string
		want   PrivacyState
	}{
		{"empty", "", PrivacyStateUnknown},
		{"whitespace_only", "   ", PrivacyStateUnknown},
		{"free_upper", "FREE", PrivacyStateFree},
		{"free_lower", "free", PrivacyStateFree},
		{"unallotted", "Unallotted", PrivacyStateFree},
		{"alloted_api_spelling", "ALLOTED", PrivacyStateAllotted},
		{"allotted_proper_spelling", "Allotted", PrivacyStateAllotted},
		{"enabled", "ENABLED", PrivacyStateAllotted},
		{"disabled", "DISABLED", PrivacyStateAllotted},
		{"used", "Used", PrivacyStateAllotted},
		{"discard", "DISCARD", PrivacyStateDiscard},
		{"discarded", "Discarded", PrivacyStateDiscard},
		{"unrecognized", "something else", PrivacyStateUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ClassifyPrivacyStatus(tc.status))
		})
	}
}

// TestPrivacyStateHelpers asserts the state-level predicates over every constant.
func TestPrivacyStateHelpers(t *testing.T) {
	t.Parallel()

	assert.True(t, PrivacyStateAllotted.IsAllotted())
	assert.False(t, PrivacyStateFree.IsAllotted())
	assert.False(t, PrivacyStateDiscard.IsAllotted())
	assert.False(t, PrivacyStateUnknown.IsAllotted())

	assert.True(t, PrivacyStateFree.IsFree())
	assert.False(t, PrivacyStateAllotted.IsFree())
	assert.False(t, PrivacyStateDiscard.IsFree())
	assert.False(t, PrivacyStateUnknown.IsFree())
}

// TestDomainPrivacyGetListEntryHelpers asserts the entry-level State/IsEnabled
// helpers, including the disabled-vs-enabled distinction the EnsureEnabled state
// machine depends on.
func TestDomainPrivacyGetListEntryHelpers(t *testing.T) {
	t.Parallel()

	enabled := DomainPrivacyGetListEntry{Status: String("ENABLED"), DomainName: String("example.com")}
	assert.Equal(t, PrivacyStateAllotted, enabled.State())
	assert.True(t, enabled.IsEnabled())

	disabled := DomainPrivacyGetListEntry{Status: String("DISABLED"), DomainName: String("example.com")}
	assert.Equal(t, PrivacyStateAllotted, disabled.State())
	assert.False(t, disabled.IsEnabled(), "'DISABLED' must not be read as enabled")

	free := DomainPrivacyGetListEntry{Status: String("FREE")}
	assert.Equal(t, PrivacyStateFree, free.State())
	assert.False(t, free.IsEnabled())

	nilStatus := DomainPrivacyGetListEntry{}
	assert.Equal(t, PrivacyStateUnknown, nilStatus.State())
	assert.False(t, nilStatus.IsEnabled())
}
