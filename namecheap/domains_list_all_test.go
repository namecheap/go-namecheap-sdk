package namecheap

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// domainsPageResponse renders a domains.getList XML response for the given page,
// pageSize and total, emitting exactly the rows that belong on that page and a
// paging block reporting total.
func domainsPageResponse(page, pageSize, total int) string {
	start := (page - 1) * pageSize
	count := total - start
	if count < 0 {
		count = 0
	}
	if count > pageSize {
		count = pageSize
	}

	var rows strings.Builder
	for i := 0; i < count; i++ {
		id := start + i + 1
		fmt.Fprintf(&rows, `<Domain ID="%d" Name="domain%d.com" User="user" Created="06/02/2021" Expires="06/02/2022" IsExpired="false" IsLocked="false" AutoRenew="true" WhoisGuard="ENABLED" IsPremium="false" IsOurDNS="false" />`, id, id)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.domains.getList">
		<DomainGetListResult>%s</DomainGetListResult>
		<Paging>
			<TotalItems>%d</TotalItems>
			<CurrentPage>%d</CurrentPage>
			<PageSize>%d</PageSize>
		</Paging>
	</CommandResponse>
</ApiResponse>`, rows.String(), total, page, pageSize)
}

// pagedDomainsServer returns a mock server that serves domainsPageResponse for
// the Page/PageSize parsed from each request body, and a pointer to a counter of
// the number of requests it has served.
func pagedDomainsServer(t *testing.T, total int) (*httptest.Server, *int64) {
	t.Helper()
	var calls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		q, _ := url.ParseQuery(string(body))
		page, _ := strconv.Atoi(q.Get("Page"))
		pageSize, _ := strconv.Atoi(q.Get("PageSize"))
		if page == 0 {
			page = 1
		}
		if pageSize == 0 {
			pageSize = 20
		}
		_, _ = w.Write([]byte(domainsPageResponse(page, pageSize, total)))
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func TestDomainsListAllSlice(t *testing.T) {
	t.Parallel()
	server, calls := pagedDomainsServer(t, 25)

	client := setupClient(nil)
	client.BaseURL = server.URL

	domains, err := client.Domains.ListAllSlice(context.Background(), &DomainsGetListArgs{PageSize: Int(10)})
	assert.NoError(t, err)

	if !assert.Len(t, domains, 25) {
		return
	}
	assert.Equal(t, "domain1.com", *domains[0].Name)
	assert.Equal(t, "domain25.com", *domains[24].Name)
	// 25 items / 10 per page = 3 pages fetched (10 + 10 + 5).
	assert.Equal(t, int64(3), atomic.LoadInt64(calls))
}

func TestDomainsListAllDefaultsToMaxPageSize(t *testing.T) {
	t.Parallel()
	var sentPageSize string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q, _ := url.ParseQuery(string(body))
		sentPageSize = q.Get("PageSize")
		_, _ = w.Write([]byte(domainsPageResponse(1, domainsMaxPageSize, 3)))
	}))
	t.Cleanup(server.Close)

	client := setupClient(nil)
	client.BaseURL = server.URL

	_, err := client.Domains.ListAllSlice(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, strconv.Itoa(domainsMaxPageSize), sentPageSize, "unset PageSize should default to the documented maximum")
}

func TestDomainsListAllDoesNotMutateArgs(t *testing.T) {
	t.Parallel()
	server, _ := pagedDomainsServer(t, 4)

	client := setupClient(nil)
	client.BaseURL = server.URL

	args := &DomainsGetListArgs{PageSize: Int(10)}
	_, err := client.Domains.ListAllSlice(context.Background(), args)
	assert.NoError(t, err)

	assert.Nil(t, args.Page, "ListAll must not set Page on the caller's args")
	assert.Equal(t, 10, *args.PageSize, "ListAll must not change the caller's PageSize")
}

// TestDomainsListAllLaziness proves endpoint-level laziness: breaking after the
// first domain of a 5-page result performs only a single HTTP request.
func TestDomainsListAllLaziness(t *testing.T) {
	t.Parallel()
	server, calls := pagedDomainsServer(t, 25)

	client := setupClient(nil)
	client.BaseURL = server.URL

	var first string
	for d, err := range client.Domains.ListAll(context.Background(), &DomainsGetListArgs{PageSize: Int(10)}) {
		if !assert.NoError(t, err) {
			break
		}
		first = *d.Name
		break
	}

	assert.Equal(t, "domain1.com", first)
	assert.Equal(t, int64(1), atomic.LoadInt64(calls), "early break should fetch only the first page")
}

// TestDomainsListAllErrorMidIteration proves a fetch error on page 2 yields
// page-1 items first, then the error, then stops.
func TestDomainsListAllErrorMidIteration(t *testing.T) {
	t.Parallel()
	var calls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&calls, 1)
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
	<Errors><Error Number="4022337">Server error</Error></Errors>
</ApiResponse>`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		q, _ := url.ParseQuery(string(body))
		page, _ := strconv.Atoi(q.Get("Page"))
		_, _ = w.Write([]byte(domainsPageResponse(page, 10, 25)))
	}))
	t.Cleanup(server.Close)

	// One attempt only, so the page-2 server error is terminal (the resilience
	// layer would otherwise retry the retryable code).
	client := NewClient(&ClientOptions{
		UserName: ncUserName, ApiUser: ncAPIUser, ApiKey: ncAPIKey, ClientIp: ncClientIP,
		RateLimit: &RateLimitOptions{Disabled: true},
		Retry:     &RetryOptions{MaxAttempts: 1},
	})
	client.BaseURL = server.URL

	var got []string
	var gotErr error
	for d, err := range client.Domains.ListAll(context.Background(), &DomainsGetListArgs{PageSize: Int(10)}) {
		if err != nil {
			gotErr = err
			break
		}
		got = append(got, *d.Name)
	}

	// Page 1 (10 items) is yielded before the page-2 error surfaces.
	if assert.Len(t, got, 10) {
		assert.Equal(t, "domain1.com", got[0])
		assert.Equal(t, "domain10.com", got[9])
	}
	assert.Error(t, gotErr)
	assert.ErrorContains(t, gotErr, "Server error")
}

// TestDomainsListAllSliceUnderRateLimiter is the rate-limiter interplay
// integration test: a 15-page ListAllSlice through the full resilience pipeline
// (limiter + concurrency gate + retry) completes without deadlock and returns
// every item. The limiter is enabled but paced high so the test runs quickly.
func TestDomainsListAllSliceUnderRateLimiter(t *testing.T) {
	t.Parallel()
	server, calls := pagedDomainsServer(t, 105)

	client := NewClient(&ClientOptions{
		UserName: ncUserName, ApiUser: ncAPIUser, ApiKey: ncAPIKey, ClientIp: ncClientIP,
		RateLimit: &RateLimitOptions{PerMinute: 6000, MaxConcurrent: 4},
	})
	client.BaseURL = server.URL

	domains, err := client.Domains.ListAllSlice(context.Background(), &DomainsGetListArgs{PageSize: Int(10)})
	assert.NoError(t, err)

	if !assert.Len(t, domains, 105) {
		return
	}
	assert.Equal(t, "domain105.com", *domains[104].Name)
	// 105 items / 10 per page = 11 pages, all paced through the limiter without deadlock.
	assert.Equal(t, int64(11), atomic.LoadInt64(calls), "105 items / 10 per page = 11 pages")
}
