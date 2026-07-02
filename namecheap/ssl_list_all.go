package namecheap

import (
	"context"
	"iter"
)

// sslMaxPageSize is the documented maximum PageSize for ssl.getList
// (docs/namecheap-api-v2.md line 829: "Min: 10, Max: 100"). ListAll requests this
// many items per page when PageSize is unset.
const sslMaxPageSize = 100

// ListAll returns a lazy, auto-paging iterator over every SSL certificate that
// matches args, fetching each page as it is consumed. It shares the semantics of
// DomainsService.ListAll: lazy paging, clean early break (pull-based, no
// goroutine leak), and a context cancelled between pages surfaced as the next
// yielded error. args is never mutated (iteration uses a shallow copy with Page
// advanced per page and PageSize defaulted to sslMaxPageSize when unset). Use
// GetListWithContext for page-level control.
//
//	for cert, err := range client.SSL.ListAll(ctx, nil) {
//	    if err != nil {
//	        return err
//	    }
//	    fmt.Println(*cert.HostName)
//	}
func (ss *SSLService) ListAll(ctx context.Context, args *SSLGetListArgs) iter.Seq2[*SSLListCertificate, error] {
	return pageAll(ctx, func(ctx context.Context, page int) ([]*SSLListCertificate, int, error) {
		return ss.fetchSSLPage(ctx, args, page)
	})
}

// ListAllSlice eagerly collects every certificate matching args into a single
// slice, draining ListAll. The result is preallocated from the first page's
// TotalItems when available. On the first fetch error it returns that error
// together with the certificates gathered from the pages that succeeded before
// it.
func (ss *SSLService) ListAllSlice(ctx context.Context, args *SSLGetListArgs) ([]*SSLListCertificate, error) {
	var total int
	seq := pageAll(ctx, func(ctx context.Context, page int) ([]*SSLListCertificate, int, error) {
		items, pageTotal, err := ss.fetchSSLPage(ctx, args, page)
		total = pageTotal
		return items, pageTotal, err
	})
	return collectAll(seq, &total)
}

// fetchSSLPage fetches one page of certificates and adapts the response to the
// pager's (items, total, err) shape.
func (ss *SSLService) fetchSSLPage(ctx context.Context, args *SSLGetListArgs, page int) ([]*SSLListCertificate, int, error) {
	resp, err := ss.GetListWithContext(ctx, sslListAllPageArgs(args, page))
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
	return ptrsOf(resp.SSLCertificates), total, nil
}

// sslListAllPageArgs returns a shallow copy of args (or a fresh value when nil)
// with Page set and PageSize defaulted to the documented maximum when unset,
// leaving the caller's args untouched.
func sslListAllPageArgs(args *SSLGetListArgs, page int) *SSLGetListArgs {
	var c SSLGetListArgs
	if args != nil {
		c = *args
	}
	c.Page = Int(page)
	if c.PageSize == nil {
		c.PageSize = Int(sslMaxPageSize)
	}
	return &c
}
