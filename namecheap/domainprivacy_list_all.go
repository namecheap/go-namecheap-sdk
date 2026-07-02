package namecheap

import (
	"context"
	"iter"
)

// privacyMaxPageSize is the documented maximum PageSize for whoisguard.getlist
// (docs/namecheap-api-v2.md line 1563: "Min: 2, Max: 100"). ListAll requests this
// many items per page when PageSize is unset.
const privacyMaxPageSize = 100

// ListAll returns a lazy, auto-paging iterator over every domain-privacy
// subscription that matches args, fetching each page as it is consumed. It shares
// the semantics of DomainsService.ListAll: lazy paging, clean early break
// (pull-based, no goroutine leak), and a context cancelled between pages surfaced
// as the next yielded error. args is never mutated (iteration uses a shallow copy
// with Page advanced per page and PageSize defaulted to privacyMaxPageSize when
// unset). Use GetListWithContext for page-level control.
//
//	for sub, err := range client.DomainPrivacy.ListAll(ctx, nil) {
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Println(*sub.DomainName)
//	}
func (dps *DomainPrivacyService) ListAll(ctx context.Context, args *DomainPrivacyGetListArgs) iter.Seq2[*DomainPrivacyGetListEntry, error] {
	return pageAll(ctx, func(ctx context.Context, page int) ([]*DomainPrivacyGetListEntry, int, error) {
		return dps.fetchPrivacyPage(ctx, args, page)
	})
}

// ListAllSlice eagerly collects every subscription matching args into a single
// slice, draining ListAll. The result is preallocated from the first page's
// TotalItems when available. On the first fetch error it returns that error
// together with the subscriptions gathered from the pages that succeeded before
// it.
func (dps *DomainPrivacyService) ListAllSlice(ctx context.Context, args *DomainPrivacyGetListArgs) ([]*DomainPrivacyGetListEntry, error) {
	var total int
	seq := pageAll(ctx, func(ctx context.Context, page int) ([]*DomainPrivacyGetListEntry, int, error) {
		items, pageTotal, err := dps.fetchPrivacyPage(ctx, args, page)
		total = pageTotal
		return items, pageTotal, err
	})
	return collectAll(seq, &total)
}

// fetchPrivacyPage fetches one page of subscriptions and adapts the response to
// the pager's (items, total, err) shape.
func (dps *DomainPrivacyService) fetchPrivacyPage(ctx context.Context, args *DomainPrivacyGetListArgs, page int) ([]*DomainPrivacyGetListEntry, int, error) {
	resp, err := dps.GetListWithContext(ctx, privacyListAllPageArgs(args, page))
	if err != nil {
		return nil, 0, err
	}
	if resp == nil {
		return nil, 0, nil
	}
	total := 0
	if resp.Paging != nil {
		total = intValue(resp.Paging.TotalItems)
	}
	return ptrsOf(resp.DomainPrivacyList), total, nil
}

// privacyListAllPageArgs returns a shallow copy of args (or a fresh value when
// nil) with Page set and PageSize defaulted to the documented maximum when
// unset, leaving the caller's args untouched.
func privacyListAllPageArgs(args *DomainPrivacyGetListArgs, page int) *DomainPrivacyGetListArgs {
	var c DomainPrivacyGetListArgs
	if args != nil {
		c = *args
	}
	c.Page = Int(page)
	if c.PageSize == nil {
		c.PageSize = Int(privacyMaxPageSize)
	}
	return &c
}
