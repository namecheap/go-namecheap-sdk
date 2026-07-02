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
	"testing"

	"github.com/stretchr/testify/assert"
)

// pageBounds returns the 0-based start index and item count for the given page.
func pageBounds(page, pageSize, total int) (start, count int) {
	start = (page - 1) * pageSize
	count = total - start
	if count < 0 {
		count = 0
	}
	if count > pageSize {
		count = pageSize
	}
	return start, count
}

// pagedServer serves render(page, pageSize, total) for the Page/PageSize parsed
// from each request body.
func pagedServer(t *testing.T, total int, render func(page, pageSize, total int) string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_, _ = w.Write([]byte(render(page, pageSize, total)))
	}))
	t.Cleanup(server.Close)
	return server
}

func envelope(commandType, result string, page, pageSize, total int) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="%s">
		%s
		<Paging>
			<TotalItems>%d</TotalItems>
			<CurrentPage>%d</CurrentPage>
			<PageSize>%d</PageSize>
		</Paging>
	</CommandResponse>
</ApiResponse>`, commandType, result, total, page, pageSize)
}

func transferPageResponse(page, pageSize, total int) string {
	start, count := pageBounds(page, pageSize, total)
	var rows strings.Builder
	for i := 0; i < count; i++ {
		id := start + i + 1
		fmt.Fprintf(&rows, `<Transfer TransferID="%d" DomainName="transfer%d.com" User="user" TransferDate="06/02/2021" OrderID="%d" StatusID="1" Status="In progress" />`, id, id, id)
	}
	result := "<TransferGetListResult>" + rows.String() + "</TransferGetListResult>"
	return envelope("namecheap.domains.transfer.getList", result, page, pageSize, total)
}

func sslPageResponse(page, pageSize, total int) string {
	start, count := pageBounds(page, pageSize, total)
	var rows strings.Builder
	for i := 0; i < count; i++ {
		id := start + i + 1
		fmt.Fprintf(&rows, `<SSL CertificateID="%d" HostName="ssl%d.com" SSLType="PositiveSSL" PurchaseDate="06/02/2021" ExpireDate="06/02/2022" Status="active" />`, id, id)
	}
	result := "<SSLListResult>" + rows.String() + "</SSLListResult>"
	return envelope("namecheap.ssl.getList", result, page, pageSize, total)
}

func privacyPageResponse(page, pageSize, total int) string {
	start, count := pageBounds(page, pageSize, total)
	var rows strings.Builder
	for i := 0; i < count; i++ {
		id := start + i + 1
		fmt.Fprintf(&rows, `<Whoisguard ID="%d" DomainName="privacy%d.com" Created="06/02/2021" Expires="06/02/2022" Status="ENABLED" />`, id, id)
	}
	result := "<WhoisguardGetListResult>" + rows.String() + "</WhoisguardGetListResult>"
	return envelope("namecheap.whoisguard.getlist", result, page, pageSize, total)
}

func TestDomainsTransferListAllSlice(t *testing.T) {
	t.Parallel()
	server := pagedServer(t, 25, transferPageResponse)
	client := setupClient(nil)
	client.BaseURL = server.URL

	transfers, err := client.DomainsTransfer.ListAllSlice(context.Background(), &DomainsTransferGetListArgs{PageSize: Int(10)})
	assert.NoError(t, err)
	if !assert.Len(t, transfers, 25) {
		return
	}
	assert.Equal(t, "transfer1.com", *transfers[0].DomainName)
	assert.Equal(t, "transfer25.com", *transfers[24].DomainName)
}

func TestSSLListAllSlice(t *testing.T) {
	t.Parallel()
	server := pagedServer(t, 25, sslPageResponse)
	client := setupClient(nil)
	client.BaseURL = server.URL

	certs, err := client.SSL.ListAllSlice(context.Background(), &SSLGetListArgs{PageSize: Int(10)})
	assert.NoError(t, err)
	if !assert.Len(t, certs, 25) {
		return
	}
	assert.Equal(t, "ssl1.com", *certs[0].HostName)
	assert.Equal(t, "ssl25.com", *certs[24].HostName)
}

func TestDomainPrivacyListAllSlice(t *testing.T) {
	t.Parallel()
	server := pagedServer(t, 25, privacyPageResponse)
	client := setupClient(nil)
	client.BaseURL = server.URL

	subs, err := client.DomainPrivacy.ListAllSlice(context.Background(), &DomainPrivacyGetListArgs{PageSize: Int(10)})
	assert.NoError(t, err)
	if !assert.Len(t, subs, 25) {
		return
	}
	assert.Equal(t, "privacy1.com", *subs[0].DomainName)
	assert.Equal(t, "privacy25.com", *subs[24].DomainName)
}

// TestUsersAddressListAll covers the flat (non-paged) iterator: a single fetch
// yields every entry, and ListAllSlice collects them.
func TestUsersAddressListAll(t *testing.T) {
	t.Parallel()
	response := `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.getList">
		<AddressGetListResult>
			<List AddressId="1" AddressName="Home" />
			<List AddressId="2" AddressName="Work" />
		</AddressGetListResult>
	</CommandResponse>
</ApiResponse>`

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	client := setupClient(nil)
	client.BaseURL = server.URL

	addresses, err := client.UsersAddress.ListAllSlice(context.Background())
	assert.NoError(t, err)
	if !assert.Len(t, addresses, 2) {
		return
	}
	assert.Equal(t, "Home", *addresses[0].AddressName)
	assert.Equal(t, "Work", *addresses[1].AddressName)
	assert.Equal(t, 1, calls, "the flat endpoint is fetched exactly once")
}

func TestUsersAddressListAllContextCancelled(t *testing.T) {
	t.Parallel()
	client := setupClient(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.UsersAddress.ListAllSlice(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
