package namecheap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// transferStatusXML renders a transfer.getStatus OK response for the given
// transfer id and free-text status description.
func transferStatusXML(transferID int, status string) string {
	return fmt.Sprintf(
		`<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<CommandResponse Type="namecheap.domains.transfer.getStatus">
				<DomainTransferGetStatusResult TransferID="%d" StatusID="5" Status="%s" />
			</CommandResponse>
		</ApiResponse>`, transferID, status)
}

func TestDomainsTransferService_WaitForCompletion(t *testing.T) {
	t.Parallel()

	t.Run("polls_until_terminal", func(t *testing.T) {
		t.Parallel()
		var calls int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := atomic.AddInt32(&calls, 1)
			status := "Transfer in progress"
			switch {
			case n == 1:
				status = "Pending"
			case n >= 3:
				status = "Transfer completed"
			}
			_, _ = w.Write([]byte(transferStatusXML(123, status)))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.WaitForCompletionWithContext(
			context.Background(), 123, WithPollInterval(time.Millisecond))
		assert.NoError(t, err)
		assert.Equal(t, TransferStateCompleted, resp.TransferState())
		assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3), "must poll through the non-terminal states before returning")
	})

	t.Run("returns_immediately_when_already_terminal", func(t *testing.T) {
		t.Parallel()
		var calls int32
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			_, _ = w.Write([]byte(transferStatusXML(123, "Transfer completed")))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.DomainsTransfer.WaitForCompletionWithContext(context.Background(), 123)
		assert.NoError(t, err)
		assert.Equal(t, TransferStateCompleted, resp.TransferState())
		assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "a terminal transfer needs exactly one poll")
	})

	t.Run("ctx_cancel_mid_poll_returns_promptly", func(t *testing.T) {
		t.Parallel()
		firstReq := make(chan struct{}, 1)
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(transferStatusXML(123, "Transfer in progress")))
			select {
			case firstReq <- struct{}{}:
			default:
			}
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		type result struct {
			resp *DomainsTransferGetStatusCommandResponse
			err  error
		}
		done := make(chan result, 1)
		go func() {
			// A long interval guarantees a prompt return is due to cancellation, not
			// the poll timer firing.
			resp, err := client.DomainsTransfer.WaitForCompletionWithContext(ctx, 123, WithPollInterval(10*time.Second))
			done <- result{resp, err}
		}()

		<-firstReq                        // first poll completed; the waiter is now between polls
		time.Sleep(10 * time.Millisecond) // give it time to enter the interval select
		start := time.Now()
		cancel()

		select {
		case res := <-done:
			assert.Less(t, time.Since(start), 100*time.Millisecond, "cancellation must return promptly, not after the poll interval")
			assert.ErrorIs(t, res.err, context.Canceled)
			assert.Nil(t, res.resp)
		case <-time.After(2 * time.Second):
			t.Fatal("WaitForCompletionWithContext did not return promptly after cancellation")
		}
	})
}
