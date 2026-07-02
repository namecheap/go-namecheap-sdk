package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const domainsTransferGetListOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<RequestedCommand>namecheap.domains.transfer.getList</RequestedCommand>
		<CommandResponse Type="namecheap.domains.transfer.getList">
			<TransferGetListResult>
				<Transfer TransferID="101" DomainName="alpha.com" User="acct" TransferDate="05/20/2026" OrderID="9001" StatusID="5" Status="Transfer in progress" />
				<Transfer TransferID="102" DomainName="beta.net" User="acct" TransferDate="05/21/2026" OrderID="9002" StatusID="20" Status="Transfer completed" />
			</TransferGetListResult>
			<Paging>
				<TotalItems>2</TotalItems>
				<CurrentPage>1</CurrentPage>
				<PageSize>10</PageSize>
			</Paging>
		</CommandResponse>
	</ApiResponse>
`

func TestDomainsTransferService_GetList(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_paging", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsTransferGetListOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.GetListWithContext(context.Background(), &DomainsTransferGetListArgs{
			ListType:   String("INPROGRESS"),
			SearchTerm: String("alpha"),
			Page:       Int(2),
			PageSize:   Int(50),
			SortBy:     String("TRANSFERDATE_DESC"),
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		// Paging/filter params serialized correctly onto the wire.
		assert.Equal(t, "namecheap.domains.transfer.getList", sentBody.Get("Command"))
		assert.Equal(t, "INPROGRESS", sentBody.Get("ListType"))
		assert.Equal(t, "alpha", sentBody.Get("SearchTerm"))
		assert.Equal(t, "2", sentBody.Get("Page"))
		assert.Equal(t, "50", sentBody.Get("PageSize"))
		assert.Equal(t, "TRANSFERDATE_DESC", sentBody.Get("SortBy"))

		transfers := *resp.Transfers
		assert.Len(t, transfers, 2)
		assert.Equal(t, 101, *transfers[0].TransferID)
		assert.Equal(t, "alpha.com", *transfers[0].DomainName)
		assert.Equal(t, "acct", *transfers[0].User)
		assert.Equal(t, "05/20/2026", *transfers[0].TransferDate)
		assert.Equal(t, 9001, *transfers[0].OrderID)
		assert.Equal(t, 5, *transfers[0].StatusID)
		assert.Equal(t, TransferStateInProgress, transfers[0].TransferState())
		assert.Equal(t, TransferStateCompleted, transfers[1].TransferState())

		assert.Equal(t, 2, *resp.Paging.TotalItems)
		assert.Equal(t, 1, *resp.Paging.CurrentPage)
		assert.Equal(t, 10, *resp.Paging.PageSize)
	})

	t.Run("nil_args_sends_only_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsTransferGetListOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsTransfer.GetListWithContext(context.Background(), nil)
		assert.NoError(t, err)
		assert.Equal(t, "namecheap.domains.transfer.getList", sentBody.Get("Command"))
		assert.Empty(t, sentBody.Get("ListType"))
		assert.Empty(t, sentBody.Get("Page"))
	})

	t.Run("invalid_list_type_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainsTransfer.GetListWithContext(context.Background(), &DomainsTransferGetListArgs{ListType: String("BOGUS")})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ListType")
	})

	t.Run("invalid_sort_by_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainsTransfer.GetListWithContext(context.Background(), &DomainsTransferGetListArgs{SortBy: String("BOGUS")})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SortBy")
	})

	t.Run("invalid_page_size_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainsTransfer.GetListWithContext(context.Background(), &DomainsTransferGetListArgs{PageSize: Int(5)})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid PageSize")
	})

	t.Run("invalid_page_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainsTransfer.GetListWithContext(context.Background(), &DomainsTransferGetListArgs{Page: Int(0)})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Page")
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(apiErrorXML(2011170, "Some error")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsTransfer.GetListWithContext(context.Background(), nil)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011170, apiErr.Number)
		}
	})
}
