package namecheap

import (
	"context"
	"iter"
)

// domainsMaxPageSize is the documented maximum PageSize for domains.getList
// (docs/namecheap-api-v2.md line 71: "Min: 10, Max: 100"). ListAll requests this
// many items per page when the caller leaves PageSize unset, minimizing the
// number of API calls made under the rate limiter.
const domainsMaxPageSize = 100

// ListAll returns a lazy, auto-paging iterator over every domain that matches
// args, transparently fetching each page as it is consumed. It is the idiomatic
// way to walk an entire portfolio without writing a manual paging loop:
//
//	for domain, err := range client.Domains.ListAll(ctx, &namecheap.DomainsGetListArgs{}) {
//	    if err != nil {
//	        return err // a fetch failed mid-iteration; iteration has stopped
//	    }
//	    fmt.Println(*domain.Name)
//	}
//
// The iterator is lazy (page N+1 is fetched only when page N is drained), a break
// stops it cleanly without fetching further pages (it is pull-based, so there is
// no goroutine to leak), and a context cancelled between pages surfaces as the
// next yielded error. args is never mutated: ListAll iterates over a shallow copy
// with Page advanced per page and PageSize defaulted to the documented maximum
// (domainsMaxPageSize) when the caller left it unset. Use GetListWithContext
// directly when you need page-level control or the raw paging metadata.
func (ds *DomainsService) ListAll(ctx context.Context, args *DomainsGetListArgs) iter.Seq2[*Domain, error] {
	return pageAll(ctx, func(ctx context.Context, page int) ([]*Domain, int, error) {
		return ds.fetchDomainsPage(ctx, args, page)
	})
}

// ListAllSlice eagerly collects every domain matching args into a single slice,
// draining ListAll. The result is preallocated from the first page's TotalItems
// when available. On the first fetch error it returns that error together with
// the domains gathered from the pages that succeeded before it. Prefer ListAll
// when the portfolio is large or you may stop early; use ListAllSlice when you
// want the whole set in memory.
func (ds *DomainsService) ListAllSlice(ctx context.Context, args *DomainsGetListArgs) ([]*Domain, error) {
	var total int
	seq := pageAll(ctx, func(ctx context.Context, page int) ([]*Domain, int, error) {
		items, pageTotal, err := ds.fetchDomainsPage(ctx, args, page)
		total = pageTotal
		return items, pageTotal, err
	})
	return collectAll(seq, &total)
}

// fetchDomainsPage fetches one page of domains and adapts the response to the
// pager's (items, total, err) shape.
func (ds *DomainsService) fetchDomainsPage(ctx context.Context, args *DomainsGetListArgs, page int) ([]*Domain, int, error) {
	resp, err := ds.GetListWithContext(ctx, domainsListAllPageArgs(args, page))
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
	return ptrsOf(resp.Domains), total, nil
}

// domainsListAllPageArgs returns a shallow copy of args (or a fresh value when
// args is nil) with Page set to page and PageSize defaulted to the documented
// maximum when unset, so the caller's args are never mutated.
func domainsListAllPageArgs(args *DomainsGetListArgs, page int) *DomainsGetListArgs {
	var c DomainsGetListArgs
	if args != nil {
		c = *args
	}
	c.Page = Int(page)
	if c.PageSize == nil {
		c.PageSize = Int(domainsMaxPageSize)
	}
	return &c
}
