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

const domainsTransferUpdateStatusOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<RequestedCommand>namecheap.domains.transfer.updateStatus</RequestedCommand>
		<CommandResponse Type="namecheap.domains.transfer.updateStatus">
			<DomainTransferUpdateStatusResult TransferID="123456" Resubmit="true" />
		</CommandResponse>
	</ApiResponse>
`

func TestDomainsTransferService_UpdateStatus(t *testing.T) {
	t.Parallel()

	t.Run("success_serializes_resubmit_true", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsTransferUpdateStatusOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.UpdateStatusWithContext(context.Background(), &DomainsTransferUpdateStatusArgs{
			TransferID: 123456,
			Resubmit:   true,
		})
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.transfer.updateStatus", sentBody.Get("Command"))
		assert.Equal(t, "123456", sentBody.Get("TransferID"))
		assert.Equal(t, "true", sentBody.Get("Resubmit"), "Resubmit must be the doc-mandated string \"true\"")

		result := resp.DomainTransferUpdateStatusResult
		assert.Equal(t, 123456, *result.TransferID)
		assert.Equal(t, true, *result.Resubmit)
	})

	t.Run("resubmit_false_serialized", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsTransferUpdateStatusOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsTransfer.UpdateStatusWithContext(context.Background(), &DomainsTransferUpdateStatusArgs{
			TransferID: 123456,
			Resubmit:   false,
		})
		assert.NoError(t, err)
		assert.Equal(t, "false", sentBody.Get("Resubmit"))
	})

	t.Run("nil_args", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.DomainsTransfer.UpdateStatusWithContext(context.Background(), nil)
		var argErr *InvalidArgumentsError
		assert.True(t, errors.As(err, &argErr))
	})

	t.Run("invalid_transfer_id_no_http", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		client.BaseURL = "http://127.0.0.1:0"

		_, err := client.DomainsTransfer.UpdateStatusWithContext(context.Background(), &DomainsTransferUpdateStatusArgs{TransferID: 0, Resubmit: true})
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

		_, err := client.DomainsTransfer.UpdateStatusWithContext(context.Background(), &DomainsTransferUpdateStatusArgs{TransferID: 999, Resubmit: true})
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 2011280, apiErr.Number)
		}
	})
}
