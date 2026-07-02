package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainsTransferGetStatusResponse is the raw envelope for
// namecheap.domains.transfer.getStatus.
type DomainsTransferGetStatusResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsTransferGetStatusCommandResponse `xml:"CommandResponse"`
}

// DomainsTransferGetStatusCommandResponse wraps the transfer.getStatus result.
type DomainsTransferGetStatusCommandResponse struct {
	DomainTransferGetStatusResult *DomainsTransferGetStatusResult `xml:"DomainTransferGetStatusResult"`
}

// DomainsTransferGetStatusResult is the status of a single transfer. Fields
// follow the transfer.getStatus response table in docs/namecheap-api-v2.md
// (lines 725-729): TransferID, StatusID and Status.
type DomainsTransferGetStatusResult struct {
	// TransferID is the unique integer identifying the transfer.
	TransferID *int `xml:"TransferID,attr"`
	// StatusID is the raw numeric status code. The API doc does not enumerate
	// these codes, so it is exposed verbatim (see TransferState).
	StatusID *int `xml:"StatusID,attr"`
	// Status is the free-text transfer status description, exposed verbatim.
	// Classify it with ClassifyTransferStatus or the response TransferState
	// helper.
	Status *string `xml:"Status,attr"`
}

// TransferState classifies the response's raw Status description into a coarse
// TransferState (see ClassifyTransferStatus). It returns TransferStateUnknown
// when the response, its result, or the Status field is nil/empty. This is the
// helper WaitForCompletionWithContext uses to decide when to stop polling.
func (r *DomainsTransferGetStatusCommandResponse) TransferState() TransferState {
	if r == nil || r.DomainTransferGetStatusResult == nil || r.DomainTransferGetStatusResult.Status == nil {
		return TransferStateUnknown
	}
	return ClassifyTransferStatus(*r.DomainTransferGetStatusResult.Status)
}

// GetStatusWithContext returns the status of the transfer identified by
// transferID. It is an idempotent read and retries on transient failures.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-transfer/get-status/
func (dts *DomainsTransferService) GetStatusWithContext(ctx context.Context, transferID int) (*DomainsTransferGetStatusCommandResponse, error) {
	if transferID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"TransferID"}, Reason: "TransferID must be a positive integer"}
	}

	params := map[string]string{
		"Command":    "namecheap.domains.transfer.getStatus",
		"TransferID": strconv.Itoa(transferID),
	}

	var response DomainsTransferGetStatusResponse
	_, err := dts.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
