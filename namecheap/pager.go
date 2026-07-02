package namecheap

import (
	"context"
	"iter"
)

// pageAll turns a page-fetching function into a lazy, auto-paging iterator that
// yields every item across every page as a single flat sequence, so callers
// never write a manual fetch loop (and never get the off-by-one on the last
// partial page).
//
// fetchPage is the one small adapter a paged endpoint supplies: given a 1-based
// page number it returns that page's items, the total item count reported by the
// endpoint's paging block (0 when the endpoint does not report one), and any
// error. See the per-endpoint ListAll methods (for example DomainsService.ListAll)
// for the mechanical shape — copy the caller's args, set Page and a max PageSize,
// call the existing GetListWithContext, and return (items, TotalItems, err).
//
// Semantics (each is exercised by the pager table tests):
//
//   - Lazy / pull-based: page N+1 is fetched only when the consumer has drained
//     page N and asks for more. range-over-func is pull-based, not channel- or
//     goroutine-based, so nothing runs ahead of consumption and there is no
//     goroutine to leak.
//   - Early break: when the consumer stops early (a break in the range loop, or
//     any code path where yield returns false) the iterator returns immediately
//     without fetching the next page. Only the pages actually consumed are
//     fetched.
//   - Multi-page: pages are fetched in order (1, 2, 3, ...) and their items are
//     yielded in order until the total is reached.
//   - Exactly one page / last partial page: iteration stops once seen >= total
//     (when a positive total is reported) so a final short page ends cleanly.
//   - Empty result: a page with zero items ends iteration (nothing is yielded).
//   - Error on page N: the items from pages 1..N-1 are yielded first, then the
//     error from page N is yielded once (with the zero value of T), then
//     iteration stops. No further pages are fetched.
//   - Context cancellation: ctx is checked before each page fetch, so a context
//     cancelled between pages surfaces as the next yielded (zero, ctx.Err())
//     pair and iteration stops. An in-flight fetch is also bound to ctx by the
//     underlying client, so its error propagates the same way.
//
// The (T, error) yield contract is: on success err is nil and the item is valid;
// on failure the item is the zero value of T and err is non-nil, and it is the
// final pair the iterator yields. Consumers should check err on every iteration.
func pageAll[T any](ctx context.Context, fetchPage func(ctx context.Context, page int) (items []T, total int, err error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		seen := 0
		for page := 1; ; page++ {
			if err := ctx.Err(); err != nil {
				var zero T
				yield(zero, err)
				return
			}
			items, total, err := fetchPage(ctx, page)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for i := range items {
				if !yield(items[i], nil) {
					// Early break: the consumer is done, so return without
					// fetching the next page (laziness).
					return
				}
			}
			seen += len(items)
			if len(items) == 0 || (total > 0 && seen >= total) {
				return
			}
		}
	}
}

// collectAll eagerly drains an auto-paging iterator into a single slice. It is
// the shared engine behind every ListAllSlice convenience method.
//
// sizeHint points at the total item count the endpoint reports (captured by the
// fetch closure on the first page, so it is populated by the time the first item
// is yielded); when it is non-nil and positive the destination slice is
// preallocated to it, so a large portfolio collects in one allocation. A nil or
// non-positive hint falls back to append's growth.
//
// On the first error collectAll stops and returns that error together with the
// items already gathered from the pages that succeeded before it (never a
// partial page: a page either contributes all its items or errors as a whole).
// A fully empty result returns a nil slice and a nil error.
func collectAll[T any](seq iter.Seq2[T, error], sizeHint *int) ([]T, error) {
	var out []T
	for item, err := range seq {
		if err != nil {
			return out, err
		}
		if out == nil {
			capHint := 0
			if sizeHint != nil && *sizeHint > 0 {
				capHint = *sizeHint
			}
			out = make([]T, 0, capHint)
		}
		out = append(out, item)
	}
	return out, nil
}

// ptrsOf returns a slice of pointers to the elements of the slice pointed to by
// s, or nil when s is nil or empty. Each returned pointer aliases the
// corresponding element of the underlying array; the pager hands every page's
// slice to a single consumer and then discards it, so aliasing is safe here. It
// bridges the endpoints' `*[]T` response fields to the `[]*T` item type the
// iterators yield.
func ptrsOf[T any](s *[]T) []*T {
	if s == nil || len(*s) == 0 {
		return nil
	}
	src := *s
	out := make([]*T, len(src))
	for i := range src {
		out[i] = &src[i]
	}
	return out
}

// intValue dereferences a *int, returning 0 for a nil pointer. It is used to read
// the optional TotalItems field of a paging block.
func intValue(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
