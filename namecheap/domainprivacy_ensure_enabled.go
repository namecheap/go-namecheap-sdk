package namecheap

import (
	"context"
	"fmt"
	"strings"
)

// ensureEnabledPageSize is the page size EnsureEnabledWithContext requests so a
// single getList call sees as many subscriptions as one page allows (the
// documented maximum). It is not full pagination; see EnsureEnabledWithContext.
const ensureEnabledPageSize = 100

// EnsureEnabledAction reports which branch of the enable/allot state machine
// EnsureEnabledWithContext took, so a caller can log or assert on the outcome.
type EnsureEnabledAction string

const (
	// EnsureEnabledAlreadyEnabled means a subscription was already attached to the
	// domain and already enabled; the call was a no-op.
	EnsureEnabledAlreadyEnabled EnsureEnabledAction = "ALREADY_ENABLED"
	// EnsureEnabledEnabled means a subscription was already attached to the domain
	// but disabled, so it was enabled.
	EnsureEnabledEnabled EnsureEnabledAction = "ENABLED"
	// EnsureEnabledAllottedAndEnabled means a FREE subscription was allotted to the
	// domain and then enabled.
	EnsureEnabledAllottedAndEnabled EnsureEnabledAction = "ALLOTTED_AND_ENABLED"
)

// DomainPrivacyEnsureEnabledResult reports the outcome of
// EnsureEnabledWithContext: which subscription ended up protecting the domain and
// what the helper had to do to get there.
type DomainPrivacyEnsureEnabledResult struct {
	// PrivacyID is the subscription that now protects the domain.
	PrivacyID int
	// Domain is the domain that was requested.
	Domain string
	// Action is the branch the state machine took (see EnsureEnabledAction).
	Action EnsureEnabledAction
}

// EnsureEnabledWithContext is a convenience helper for the common "just turn
// privacy on for this domain" case. It composes the domain-privacy state machine
// so callers do not have to reason about the attach-vs-activate distinction
// themselves.
//
// The attach-vs-activate distinction. A privacy subscription must first be
// ALLOTTED (attached) to a domain, and separately ENABLED (activated) with a
// forwarding email. AllotWithContext does the first; EnableWithContext does the
// second. This helper hides both behind one call.
//
// State machine. It reads the subscription list once (getList, ListType=ALL) and
// then, depending on the domain's starting state:
//
//   - already attached and enabled  -> no-op (EnsureEnabledAlreadyEnabled);
//   - attached but disabled         -> enable it (EnsureEnabledEnabled);
//   - not attached, a FREE one exists -> allot it to the domain, then enable it
//     (EnsureEnabledAllottedAndEnabled);
//   - not attached, no FREE one     -> ErrNoFreePrivacySubscription.
//
// Scope. The read is a single page of up to ensureEnabledPageSize subscriptions
// (not full pagination); an account with more subscriptions than one page may
// need the lower-level primitives. domain and forwardedToEmail are required and
// reported together as an *InvalidArgumentsError before any request is sent.
// Each underlying mutation keeps its own retry semantics (allot and enable are
// idempotent and retry on transient failures).
func (dps *DomainPrivacyService) EnsureEnabledWithContext(ctx context.Context, domain, forwardedToEmail string) (*DomainPrivacyEnsureEnabledResult, error) {
	missing := make([]string, 0, 2)
	if domain == "" {
		missing = append(missing, "DomainName")
	}
	if forwardedToEmail == "" {
		missing = append(missing, "ForwardedToEmail")
	}
	if len(missing) > 0 {
		return nil, &InvalidArgumentsError{Fields: missing}
	}

	list, err := dps.GetListWithContext(ctx, &DomainPrivacyGetListArgs{
		ListType: String("ALL"),
		Page:     Int(1),
		PageSize: Int(ensureEnabledPageSize),
	})
	if err != nil {
		return nil, err
	}

	target, free := findPrivacySubscription(list, domain)

	// The domain already has a subscription attached.
	if target != nil {
		if target.ID == nil {
			return nil, fmt.Errorf("domain privacy subscription for %q is missing its ID", domain)
		}
		if target.IsEnabled() {
			return &DomainPrivacyEnsureEnabledResult{PrivacyID: *target.ID, Domain: domain, Action: EnsureEnabledAlreadyEnabled}, nil
		}
		if _, err := dps.EnableWithContext(ctx, *target.ID, forwardedToEmail); err != nil {
			return nil, err
		}
		return &DomainPrivacyEnsureEnabledResult{PrivacyID: *target.ID, Domain: domain, Action: EnsureEnabledEnabled}, nil
	}

	// No subscription for the domain yet: allot a free one, then enable it.
	if free == nil || free.ID == nil {
		return nil, ErrNoFreePrivacySubscription
	}
	if _, err := dps.AllotWithContext(ctx, *free.ID, domain); err != nil {
		return nil, err
	}
	if _, err := dps.EnableWithContext(ctx, *free.ID, forwardedToEmail); err != nil {
		return nil, err
	}
	return &DomainPrivacyEnsureEnabledResult{PrivacyID: *free.ID, Domain: domain, Action: EnsureEnabledAllottedAndEnabled}, nil
}

// findPrivacySubscription scans a getList response for the subscription already
// attached to domain (target) and, failing that, the first FREE subscription
// available to allot (free). DISCARD entries are ignored entirely. Either return
// may be nil.
func findPrivacySubscription(list *DomainPrivacyGetListCommandResponse, domain string) (target, free *DomainPrivacyGetListEntry) {
	if list == nil || list.DomainPrivacyList == nil {
		return nil, nil
	}
	entries := *list.DomainPrivacyList
	for i := range entries {
		entry := &entries[i]
		if entry.State() == PrivacyStateDiscard {
			continue
		}
		if entry.DomainName != nil && strings.EqualFold(*entry.DomainName, domain) {
			return entry, nil
		}
		if free == nil && entry.State() == PrivacyStateFree {
			free = entry
		}
	}
	return nil, free
}
