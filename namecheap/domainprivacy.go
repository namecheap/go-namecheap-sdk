package namecheap

import (
	"errors"
	"strings"
)

// DomainPrivacyService groups the domain-privacy API commands: listing
// subscriptions (GetListWithContext), turning protection on and off
// (EnableWithContext, DisableWithContext), attaching and detaching a
// subscription to a domain (AllotWithContext, UnallotWithContext), throwing away
// an unused subscription (DiscardWithContext) and rotating the forwarding email
// (ChangeEmailAddressWithContext). EnsureEnabledWithContext composes getList →
// allot → enable for the common "just turn privacy on for this domain" case.
//
// Naming: whoisguard → domainprivacy. This service is named for the current
// product term ("domain privacy"), but the underlying Namecheap API commands
// still use the legacy "whoisguard" names. The mapping is one-to-one:
//
//	DomainPrivacy.GetListWithContext            -> namecheap.whoisguard.getlist
//	DomainPrivacy.EnableWithContext             -> namecheap.whoisguard.enable
//	DomainPrivacy.DisableWithContext            -> namecheap.whoisguard.disable
//	DomainPrivacy.AllotWithContext              -> namecheap.whoisguard.allot
//	DomainPrivacy.UnallotWithContext            -> namecheap.whoisguard.unallot
//	DomainPrivacy.DiscardWithContext            -> namecheap.whoisguard.discard
//	DomainPrivacy.ChangeEmailAddressWithContext -> namecheap.whoisguard.changeemailaddress
//
// A privacy subscription is its own entity with a numeric ID (a "WhoisguardID"
// on the wire), a status and an expiry, and is attachable to a domain. Every
// method takes that ID as a typed int (privacyID), never a string. The wire
// parameter is still named WhoisguardID for backward compatibility.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/whoisguard/
type DomainPrivacyService service

// PrivacyState is a coarse, typed classification of a domain-privacy
// subscription's lifecycle/allotment state.
//
// Grounding and the documented gap. docs/namecheap-api-v2.md (domainprivacy
// section, getlist lines 1551-1575) does NOT enumerate the values of the getList
// Status field; it only documents the getList ListType filter vocabulary — ALL |
// ALLOTED | FREE | DISCARD (line 1567). This SDK therefore does NOT invent a
// status code table. Instead it always exposes the raw Status verbatim on every
// entry (see DomainPrivacyGetListEntry.Status) and offers this small state whose
// constants are grounded in that documented ListType vocabulary.
// ClassifyPrivacyStatus maps a raw Status string onto one of these states by
// case-insensitive keyword matching.
//
// Note the spelling: the API's ListType filter value is the nonstandard
// "ALLOTED" (see allowedPrivacyListTypeValues), while this classification
// constant uses the correct English "ALLOTTED"; the classifier keys off the
// "allot" substring so it recognises both.
type PrivacyState string

const (
	// PrivacyStateUnknown is used when a Status is empty or cannot be classified.
	PrivacyStateUnknown PrivacyState = "UNKNOWN"
	// PrivacyStateFree mirrors the documented getList category FREE: the
	// subscription is not attached to any domain and is available to allot.
	PrivacyStateFree PrivacyState = "FREE"
	// PrivacyStateAllotted mirrors the documented getList category ALLOTED: the
	// subscription is attached to a domain (whether privacy is currently enabled
	// or disabled — that dimension is read separately via
	// DomainPrivacyGetListEntry.IsEnabled).
	PrivacyStateAllotted PrivacyState = "ALLOTTED"
	// PrivacyStateDiscard mirrors the documented getList category DISCARD: the
	// subscription has been discarded.
	PrivacyStateDiscard PrivacyState = "DISCARD"
)

// ClassifyPrivacyStatus maps a getList Status description onto a PrivacyState by
// case-insensitive keyword matching against the documented getList ListType
// category vocabulary (ALL | ALLOTED | FREE | DISCARD, line 1567).
//
// Because the doc enumerates no Status values, classification keys off the
// free-text description rather than a fabricated code table:
//   - a description containing "discard"               -> PrivacyStateDiscard;
//   - a description containing "free" or "unallot"     -> PrivacyStateFree;
//   - a description containing "allot", "enabled",
//     "disabled" or "used"                             -> PrivacyStateAllotted;
//   - anything else (including an empty description)    -> PrivacyStateUnknown.
//
// The enabled/disabled dimension is deliberately NOT collapsed into this state:
// both an enabled and a disabled subscription are ALLOTTED. Read the on/off
// state with DomainPrivacyGetListEntry.IsEnabled.
func ClassifyPrivacyStatus(status string) PrivacyState {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case normalized == "":
		return PrivacyStateUnknown
	case strings.Contains(normalized, "discard"):
		return PrivacyStateDiscard
	case strings.Contains(normalized, "free"), strings.Contains(normalized, "unallot"):
		return PrivacyStateFree
	case strings.Contains(normalized, "allot"),
		strings.Contains(normalized, "enabled"),
		strings.Contains(normalized, "disabled"),
		strings.Contains(normalized, "used"):
		return PrivacyStateAllotted
	default:
		return PrivacyStateUnknown
	}
}

// IsAllotted reports whether the state is ALLOTTED (attached to a domain).
func (s PrivacyState) IsAllotted() bool { return s == PrivacyStateAllotted }

// IsFree reports whether the state is FREE (unallotted and available to attach).
func (s PrivacyState) IsFree() bool { return s == PrivacyStateFree }

// ErrNoFreePrivacySubscription is returned by EnsureEnabledWithContext when the
// account holds no FREE (unallotted) domain-privacy subscription to allot to the
// requested domain and none is already attached to it. It is a client-side
// sentinel (not an *APIError); match it with errors.Is:
//
//	if errors.Is(err, namecheap.ErrNoFreePrivacySubscription) {
//	    // buy privacy first (via domains.create free-privacy flag / account purchase)
//	}
var ErrNoFreePrivacySubscription = errors.New("no free (unallotted) domain privacy subscription available to allot")
