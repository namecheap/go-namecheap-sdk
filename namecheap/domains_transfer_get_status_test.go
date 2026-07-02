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

const domainsTransferGetStatusOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<RequestedCommand>namecheap.domains.transfer.getStatus</RequestedCommand>
		<CommandResponse Type="namecheap.domains.transfer.getStatus">
			<DomainTransferGetStatusResult TransferID="123456" StatusID="5" Status="Transfer in progress, awaiting EPP code" />
		</CommandResponse>
	</ApiResponse>
`

func TestDomainsTransferService_GetStatus(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsTransferGetStatusOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.GetStatusWithContext(context.Background(), 123456)
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.transfer.getStatus", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("TransferID"))

		result := resp.DomainTransferGetStatusResult
		assert.Equal(t, 123456, *result.TransferID)
		assert.Equal(t, 5, *result.StatusID)
		assert.Equal(t, "Transfer in progress, awaiting EPP code", *result.Status)

		// The response helper classifies the raw description without inventing codes.
		assert.Equal(t, TransferStateInProgress, resp.TransferState())
		assert.False(t, resp.TransferState().IsTerminal())
		assert.True(t, resp.TransferState().IsActionRequired())
	})

	t.Run("invalid_transfer_id_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainsTransfer.GetStatusWithContext(context.Background(), 0)
		var argErr *InvalidArgumentsError
		if assert.True(t, errors.As(err, &argErr)) {
			assert.Contains(t, argErr.Fields, "TransferID")
		}
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(apiErrorXML(2011280, "TransferID is invalid")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsTransfer.GetStatusWithContext(context.Background(), 999)
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011280, apiErr.Number)
		}
	})

	t.Run("nil_safe_transfer_state", func(t *testing.T) {
		t.Parallel()
		var resp *DomainsTransferGetStatusCommandResponse
		assert.Equal(t, TransferStateUnknown, resp.TransferState())
		assert.Equal(t, TransferStateUnknown, (&DomainsTransferGetStatusCommandResponse{}).TransferState())
	})
}
