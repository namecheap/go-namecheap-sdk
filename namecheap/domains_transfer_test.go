package namecheap

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClassifyTransferStatus table-tests the description classifier and the
// IsTerminal / IsActionRequired helpers over representative getStatus
// descriptions. The classifier is grounded in the documented getList category
// vocabulary (ALL | INPROGRESS | CANCELLED | COMPLETED); the doc enumerates no
// numeric StatusID codes, so nothing here keys off a fabricated code table.
func TestClassifyTransferStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		status         string
		want           TransferState
		terminal       bool
		actionRequired bool
	}{
		{"empty", "", TransferStateUnknown, false, false},
		{"whitespace_only", "   ", TransferStateUnknown, false, false},
		{"completed_lower", "transfer completed successfully", TransferStateCompleted, true, false},
		{"completed_mixedcase", "Completed", TransferStateCompleted, true, false},
		{"cancelled_spelling", "Transfer cancelled by user", TransferStateCancelled, true, false},
		{"canceled_spelling", "Order was canceled", TransferStateCancelled, true, false},
		{"awaiting_epp", "Awaiting EPP code from registrant", TransferStateInProgress, false, true},
		{"registry_lock", "Domain is locked at the registry", TransferStateInProgress, false, true},
		{"needs_resubmit", "Please resubmit the transfer order", TransferStateInProgress, false, true},
		{"generic_in_progress", "Transfer in progress", TransferStateInProgress, false, true},
		{"pending", "Pending", TransferStateInProgress, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyTransferStatus(tc.status)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.terminal, got.IsTerminal(), "IsTerminal")
			assert.Equal(t, tc.actionRequired, got.IsActionRequired(), "IsActionRequired")
		})
	}
}

// TestTransferStateHelpersOverStates asserts the state-level helpers directly on
// every constant.
func TestTransferStateHelpersOverStates(t *testing.T) {
	t.Parallel()

	assert.True(t, TransferStateCompleted.IsTerminal())
	assert.True(t, TransferStateCancelled.IsTerminal())
	assert.False(t, TransferStateInProgress.IsTerminal())
	assert.False(t, TransferStateUnknown.IsTerminal())

	assert.True(t, TransferStateInProgress.IsActionRequired())
	assert.False(t, TransferStateCompleted.IsActionRequired())
	assert.False(t, TransferStateCancelled.IsActionRequired())
	assert.False(t, TransferStateUnknown.IsActionRequired())
}

// TestEPPCodeRedaction drives transfer.create with a known EPP code through both
// observability surfaces (hooks + an slog capture) and asserts the EPP value
// never reaches either surface while the "***" marker does. This is the same
// grep-all-observable-output pattern the #113 redaction test establishes.
func TestEPPCodeRedaction(t *testing.T) {
	const secretEPP = "EPP-SECRET-7f3Xq9Z"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(domainsTransferCreateOKResponse))
	}))
	defer server.Close()

	var mu sync.Mutex
	var seen []string
	collect := func(ss ...string) {
		mu.Lock()
		seen = append(seen, ss...)
		mu.Unlock()
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client := NewClient(&ClientOptions{
		UserName: "acct-user", ApiUser: "acct-user", ApiKey: "acct-key", ClientIp: "10.0.0.1",
		RateLimit: &RateLimitOptions{Disabled: true},
		Retry:     &RetryOptions{MaxAttempts: 1},
		Logger:    logger,
		OnRequest: func(info RequestInfo) {
			collect(info.Command)
			for k, v := range info.Params {
				collect(k, v)
			}
		},
		OnResponse: func(info ResponseInfo) {
			collect(info.Command)
			if info.Err != nil {
				collect(info.Err.Error())
			}
		},
	})
	client.BaseURL = server.URL

	_, err := client.DomainsTransfer.CreateWithContext(context.Background(), &DomainsTransferCreateArgs{
		DomainName: "example.com",
		Years:      1,
		EPPCode:    secretEPP,
	})
	assert.NoError(t, err)

	mu.Lock()
	hookBlob := strings.Join(seen, "\x00")
	mu.Unlock()
	logBlob := logBuf.String()
	all := hookBlob + "\x00" + logBlob

	assert.NotContains(t, all, secretEPP, "the EPP code must never reach an observability surface")
	assert.Contains(t, hookBlob, redactedValue, "hooks must show the redaction marker for EPPCode")
	assert.Contains(t, logBlob, redactedValue, "slog output must show the redaction marker for EPPCode")
	// The redacted key must still be present so observers see the parameter exists.
	assert.Contains(t, hookBlob, "EPPCode")
}
