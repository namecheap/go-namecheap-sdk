package namecheap

import (
	"context"
	"time"
)

// defaultTransferPollInterval is the polling cadence WaitForCompletionWithContext
// uses when no WithPollInterval option is supplied. Domain transfers move on the
// order of hours to days, so a slow default keeps API usage well within the
// rate limit.
const defaultTransferPollInterval = 30 * time.Second

// transferWaitConfig holds the resolved options for WaitForCompletionWithContext.
type transferWaitConfig struct {
	interval time.Duration
}

// TransferWaitOption configures WaitForCompletionWithContext.
type TransferWaitOption func(*transferWaitConfig)

// WithPollInterval sets how often WaitForCompletionWithContext polls GetStatus.
// A non-positive interval is ignored and the default (defaultTransferPollInterval)
// is used instead.
func WithPollInterval(d time.Duration) TransferWaitOption {
	return func(c *transferWaitConfig) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WaitForCompletionWithContext polls GetStatus for transferID until the transfer
// reaches a terminal state (COMPLETED or CANCELLED, per TransferState.IsTerminal)
// and returns the terminal getStatus response.
//
// It polls immediately, then every interval (WithPollInterval; default
// defaultTransferPollInterval). It respects ctx cancellation mid-poll: while
// waiting between polls it selects on the interval timer versus ctx.Done, so a
// cancelled or expired context returns ctx.Err() promptly rather than after the
// full interval. A GetStatus error (including a context error raised during the
// HTTP call) is returned immediately.
func (dts *DomainsTransferService) WaitForCompletionWithContext(ctx context.Context, transferID int, opts ...TransferWaitOption) (*DomainsTransferGetStatusCommandResponse, error) {
	cfg := transferWaitConfig{interval: defaultTransferPollInterval}
	for _, opt := range opts {
		opt(&cfg)
	}

	for {
		resp, err := dts.GetStatusWithContext(ctx, transferID)
		if err != nil {
			return nil, err
		}
		if resp.TransferState().IsTerminal() {
			return resp, nil
		}

		timer := time.NewTimer(cfg.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
