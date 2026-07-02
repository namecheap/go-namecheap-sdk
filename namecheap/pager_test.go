package namecheap

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
)

// errScripted is the sentinel error a scripted pager returns for a failing page.
var errScripted = errors.New("scripted page error")

// scriptedPager is a fake fetchPage source for the shared-pager table tests. It
// serves predetermined pages, reports a fixed total, optionally errors on a
// chosen page, and counts how many times it was called so laziness can be
// asserted. It is driven from a single goroutine (the pager is pull-based), so no
// synchronization is needed.
type scriptedPager struct {
	pages [][]int // pages[i] is the 0-based content of page i+1
	total int     // TotalItems reported on every page
	errAt int     // 1-based page that returns errScripted; 0 disables
	calls int     // number of fetch invocations
}

func (s *scriptedPager) fetch(_ context.Context, page int) ([]int, int, error) {
	s.calls++
	if s.errAt != 0 && page == s.errAt {
		return nil, s.total, errScripted
	}
	if page < 1 || page > len(s.pages) {
		return nil, s.total, nil
	}
	return s.pages[page-1], s.total, nil
}

// drain fully consumes an iterator, returning the items and the first error.
func drain[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var items []T
	for v, err := range seq {
		if err != nil {
			return items, err
		}
		items = append(items, v)
	}
	return items, nil
}

func TestPageAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pages     [][]int
		total     int
		errAt     int
		wantItems []int
		wantErr   error
		wantCalls int
	}{
		{
			name:      "multi_page",
			pages:     [][]int{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}},
			total:     10,
			wantItems: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			wantCalls: 5,
		},
		{
			name:      "exactly_one_page",
			pages:     [][]int{{1, 2}},
			total:     2,
			wantItems: []int{1, 2},
			wantCalls: 1,
		},
		{
			name:      "empty_result",
			pages:     [][]int{{}},
			total:     0,
			wantItems: nil,
			wantCalls: 1,
		},
		{
			name:      "last_page_partial",
			pages:     [][]int{{1, 2}, {3, 4}, {5}},
			total:     5,
			wantItems: []int{1, 2, 3, 4, 5},
			wantCalls: 3,
		},
		{
			name:      "error_on_page_3_of_5",
			pages:     [][]int{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}},
			total:     10,
			errAt:     3,
			wantItems: []int{1, 2, 3, 4}, // pages 1-2 yielded, then the error
			wantErr:   errScripted,
			wantCalls: 3, // pages 4 and 5 are never fetched
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sp := &scriptedPager{pages: tt.pages, total: tt.total, errAt: tt.errAt}
			items, err := drain(pageAll(context.Background(), sp.fetch))

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantItems, items)
			assert.Equal(t, tt.wantCalls, sp.calls)
		})
	}
}

// TestPageAllLaziness proves the iterator is pull-based: breaking after the first
// item fetches only page 1 of a 5-page result.
func TestPageAllLaziness(t *testing.T) {
	t.Parallel()
	sp := &scriptedPager{
		pages: [][]int{{1, 2}, {3, 4}, {5, 6}, {7, 8}, {9, 10}},
		total: 10,
	}

	var got []int
	for v, err := range pageAll(context.Background(), sp.fetch) {
		assert.NoError(t, err)
		got = append(got, v)
		break
	}

	assert.Equal(t, []int{1}, got)
	assert.Equal(t, 1, sp.calls, "only page 1 should be fetched when the consumer breaks early")
}

// TestPageAllContextCancellation proves a context cancelled between pages surfaces
// as the next yielded error and stops iteration before the next fetch.
func TestPageAllContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	sp := &scriptedPager{
		pages: [][]int{{1, 2}, {3, 4}, {5, 6}},
		total: 6,
	}
	// Cancel the context as a side effect of serving page 1 so the pager's
	// pre-fetch ctx check trips before page 2.
	fetch := func(ctx context.Context, page int) ([]int, int, error) {
		items, total, err := sp.fetch(ctx, page)
		if page == 1 {
			cancel()
		}
		return items, total, err
	}

	var got []int
	var gotErr error
	for v, err := range pageAll(ctx, fetch) {
		if err != nil {
			gotErr = err
			break
		}
		got = append(got, v)
	}

	assert.Equal(t, []int{1, 2}, got, "page 1 items are yielded before cancellation is observed")
	assert.ErrorIs(t, gotErr, context.Canceled)
	assert.Equal(t, 1, sp.calls, "page 2 is never fetched after cancellation")
}

// TestCollectAllPreallocatesFromTotal proves collectAll sizes the destination
// slice from the total hint captured by the fetch closure (single allocation).
func TestCollectAllPreallocatesFromTotal(t *testing.T) {
	t.Parallel()
	var total int
	sp := &scriptedPager{
		pages: [][]int{{1, 2}, {3, 4}, {5, 6}},
		total: 6,
	}
	seq := pageAll(context.Background(), func(ctx context.Context, page int) ([]int, int, error) {
		items, pageTotal, err := sp.fetch(ctx, page)
		total = pageTotal
		return items, pageTotal, err
	})

	out, err := collectAll(seq, &total)
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, out)
	assert.Equal(t, 6, cap(out), "slice should be preallocated to TotalItems and never grow")
}

// TestCollectAllReturnsPartialOnError proves collectAll returns the items it
// gathered before the failing page together with the error.
func TestCollectAllReturnsPartialOnError(t *testing.T) {
	t.Parallel()
	var total int
	sp := &scriptedPager{
		pages: [][]int{{1, 2}, {3, 4}, {5, 6}},
		total: 6,
		errAt: 2,
	}
	seq := pageAll(context.Background(), func(ctx context.Context, page int) ([]int, int, error) {
		items, pageTotal, err := sp.fetch(ctx, page)
		total = pageTotal
		return items, pageTotal, err
	})

	out, err := collectAll(seq, &total)
	assert.ErrorIs(t, err, errScripted)
	assert.Equal(t, []int{1, 2}, out, "items from the page(s) that succeeded are returned with the error")
}

func TestPtrsOf(t *testing.T) {
	t.Parallel()
	t.Run("nil_pointer", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, ptrsOf[int](nil))
	})
	t.Run("empty_slice", func(t *testing.T) {
		t.Parallel()
		empty := []int{}
		assert.Nil(t, ptrsOf(&empty))
	})
	t.Run("aliases_elements", func(t *testing.T) {
		t.Parallel()
		src := []int{10, 20, 30}
		got := ptrsOf(&src)
		if !assert.Len(t, got, 3) {
			return
		}
		assert.Equal(t, 10, *got[0])
		assert.Equal(t, 20, *got[1])
		assert.Equal(t, 30, *got[2])
	})
}
