package namecheap

import "strings"

// DomainsTransferService groups the namecheap.domains.transfer.* API commands:
// initiating an inbound domain transfer (CreateWithContext), tracking a single
// transfer (GetStatusWithContext), re-submitting a transfer after releasing the
// registry lock (UpdateStatusWithContext) and listing transfers
// (GetListWithContext). WaitForCompletionWithContext polls GetStatus until the
// transfer reaches a terminal state.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-transfer/
type DomainsTransferService service

// TransferState is a coarse, typed classification of a domain-transfer status.
//
// Grounding and the documented gap. The Namecheap API doc
// (docs/namecheap-api-v2.md, domains.transfer section, lines 681-781) does NOT
// enumerate the numeric StatusID codes returned by transfer.create /
// transfer.getStatus / transfer.getList: getStatus returns a raw StatusID (int)
// and a free-text Status description, and the doc gives no code→meaning table.
// This SDK therefore does NOT invent numeric constants. Instead it exposes the
// raw StatusID and Status verbatim on every response and offers this small,
// honest state machine whose constants are grounded in the ONE documented
// vocabulary: the getList ListType categories ALL | INPROGRESS | CANCELLED |
// COMPLETED (line 765). ClassifyTransferStatus maps a free-text description onto
// one of these states by case-insensitive keyword matching.
type TransferState string

const (
	// TransferStateUnknown is used when a status description is empty or cannot be
	// classified. It is never terminal and never action-required.
	TransferStateUnknown TransferState = "UNKNOWN"
	// TransferStateInProgress mirrors the documented getList category INPROGRESS:
	// the transfer is under way and has not reached a terminal outcome.
	TransferStateInProgress TransferState = "INPROGRESS"
	// TransferStateCompleted mirrors the documented getList category COMPLETED:
	// the transfer finished successfully. Terminal.
	TransferStateCompleted TransferState = "COMPLETED"
	// TransferStateCancelled mirrors the documented getList category CANCELLED:
	// the transfer was cancelled or failed. Terminal.
	TransferStateCancelled TransferState = "CANCELLED"
)

// ClassifyTransferStatus maps a getStatus Status description onto a TransferState
// by case-insensitive keyword matching against the documented getList category
// vocabulary (ALL | INPROGRESS | CANCELLED | COMPLETED, line 765).
//
// Because the doc enumerates no numeric StatusID codes, classification keys off
// the free-text description rather than a fabricated code table:
//   - a description containing "complete" -> TransferStateCompleted;
//   - a description containing "cancel"   -> TransferStateCancelled;
//   - any other non-empty description     -> TransferStateInProgress;
//   - an empty description                -> TransferStateUnknown.
//
// It is deliberately conservative: only the two documented terminal categories
// are recognised as terminal, and everything else in flight is InProgress.
func ClassifyTransferStatus(status string) TransferState {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case normalized == "":
		return TransferStateUnknown
	case strings.Contains(normalized, "complete"):
		return TransferStateCompleted
	case strings.Contains(normalized, "cancel"):
		return TransferStateCancelled
	default:
		return TransferStateInProgress
	}
}

// IsTerminal reports whether the state is a documented terminal outcome
// (COMPLETED or CANCELLED). A terminal transfer will not change further, so
// WaitForCompletionWithContext stops polling once IsTerminal is true.
func (s TransferState) IsTerminal() bool {
	return s == TransferStateCompleted || s == TransferStateCancelled
}

// IsActionRequired is a best-effort signal that the account holder may need to
// act to move the transfer forward (release the EPP/auth code, release the
// registry lock, or resubmit the order).
//
// Heuristic and the documented gap. The doc enumerates no numeric StatusID
// codes, and every getStatus description that asks the user to act — those
// mentioning the EPP/auth code, a registry lock, or a resubmit — classifies to
// TransferStateInProgress (it is neither "complete" nor "cancel"). This method
// therefore reports true for exactly the InProgress state: any non-terminal,
// non-unknown transfer is treated as possibly needing attention. It is a hint,
// not a guarantee; consumers that need certainty should read the raw Status
// description exposed on the response.
func (s TransferState) IsActionRequired() bool {
	return s == TransferStateInProgress
}
