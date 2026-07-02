package namecheap

import (
	"context"
	"iter"
)

// ListAll returns an iterator over every address-book entry on the account.
//
// Unlike the paged endpoints, users.address.getList is a flat, non-paged list
// (docs/namecheap-api-v2.md lines 1412-1428: no request parameters and no paging
// block), so ListAll performs exactly one fetch and yields each returned entry;
// there is no page N+1 to fetch lazily. It is provided for API uniformity with
// the paged services' ListAll iterators. The (entry, error) yield contract
// matches the paged iterators: on a fetch error a single (nil, err) pair is
// yielded and iteration stops; a caller's early break stops iteration cleanly.
// context cancellation surfaces as that error before the fetch is attempted.
func (uas *UsersAddressService) ListAll(ctx context.Context) iter.Seq2[*UsersAddressListEntry, error] {
	return func(yield func(*UsersAddressListEntry, error) bool) {
		if err := ctx.Err(); err != nil {
			yield(nil, err)
			return
		}
		resp, err := uas.GetListWithContext(ctx)
		if err != nil {
			yield(nil, err)
			return
		}
		if resp == nil {
			return
		}
		for _, entry := range ptrsOf(resp.AddressGetListResult) {
			if !yield(entry, nil) {
				return
			}
		}
	}
}

// ListAllSlice eagerly collects every address-book entry into a single slice by
// draining ListAll. Because the endpoint is non-paged this is a single fetch. It
// returns the first error encountered together with whatever was collected before
// it (for a single fetch that is nil).
func (uas *UsersAddressService) ListAllSlice(ctx context.Context) ([]*UsersAddressListEntry, error) {
	return collectAll(uas.ListAll(ctx), nil)
}
