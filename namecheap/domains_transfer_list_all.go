package namecheap

import (
	"context"
	"iter"
)

// transferMaxPageSize is the documented maximum PageSize for transfer.getList
// (docs/namecheap-api-v2.md line 768: "Min: 10, Max: 100"). ListAll requests this
// many items per page when PageSize is unset.
const transferMaxPageSize = 100

// ListAll returns a lazy, auto-paging iterator over every domain transfer that
// matches args, fetching each page as it is consumed. It shares the semantics of
// DomainsService.ListAll: lazy paging, clean early break (pull-based, no
// goroutine leak), and a context cancelled between pages surfaced as the next
// yielded error. args is never mutated (iteration uses a shallow copy with Page
// advanced per page and PageSize defaulted to transferMaxPageSize when unset).
// Use GetListWithContext for page-level control.
//
//	for transfer, err := range client.DomainsTransfer.ListAll(ctx, nil) {
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Println(*transfer.DomainName)
//	}
func (dts *DomainsTransferService) ListAll(ctx context.Context, args *DomainsTransferGetListArgs) iter.Seq2[*DomainTransfer, error] {
	return pageAll(ctx, func(ctx context.Context, page int) ([]*DomainTransfer, int, error) {
		return dts.fetchTransferPage(ctx, args, page)
	})
}

// ListAllSlice eagerly collects every transfer matching args into a single
// slice, draining ListAll. The result is preallocated from the first page's
// TotalItems when available. On the first fetch error it returns that error
// together with the transfers gathered from the pages that succeeded before it.
func (dts *DomainsTransferService) ListAllSlice(ctx context.Context, args *DomainsTransferGetListArgs) ([]*DomainTransfer, error) {
	var total int
	seq := pageAll(ctx, func(ctx context.Context, page int) ([]*DomainTransfer, int, error) {
		items, pageTotal, err := dts.fetchTransferPage(ctx, args, page)
		total = pageTotal
		return items, pageTotal, err
	})
	return collectAll(seq, &total)
}

// fetchTransferPage fetches one page of transfers and adapts the response to the
// pager's (items, total, err) shape.
func (dts *DomainsTransferService) fetchTransferPage(ctx context.Context, args *DomainsTransferGetListArgs, page int) ([]*DomainTransfer, int, error) {
	resp, err := dts.GetListWithContext(ctx, transferListAllPageArgs(args, page))
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
	return ptrsOf(resp.Transfers), total, nil
}

// transferListAllPageArgs returns a shallow copy of args (or a fresh value when
// nil) with Page set and PageSize defaulted to the documented maximum when
// unset, leaving the caller's args untouched.
func transferListAllPageArgs(args *DomainsTransferGetListArgs, page int) *DomainsTransferGetListArgs {
	var c DomainsTransferGetListArgs
	if args != nil {
		c = *args
	}
	c.Page = Int(page)
	if c.PageSize == nil {
		c.PageSize = Int(transferMaxPageSize)
	}
	return &c
}
