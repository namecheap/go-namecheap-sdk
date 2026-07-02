package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainsTransferUpdateStatusResponse is the raw envelope for
// namecheap.domains.transfer.updateStatus.
type DomainsTransferUpdateStatusResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsTransferUpdateStatusCommandResponse `xml:"CommandResponse"`
}

// DomainsTransferUpdateStatusCommandResponse wraps the transfer.updateStatus
// result.
type DomainsTransferUpdateStatusCommandResponse struct {
	DomainTransferUpdateStatusResult *DomainsTransferUpdateStatusResult `xml:"DomainTransferUpdateStatusResult"`
}

// DomainsTransferUpdateStatusResult is the outcome of a status update. Fields
// follow the transfer.updateStatus response table in docs/namecheap-api-v2.md
// (lines 749-751): TransferID and Resubmit.
type DomainsTransferUpdateStatusResult struct {
	// TransferID is the unique integer identifying the transfer.
	TransferID *int `xml:"TransferID,attr"`
	// Resubmit reports whether the transfer order was resubmitted.
	Resubmit *bool `xml:"Resubmit,attr"`
}

// DomainsTransferUpdateStatusArgs are the arguments for UpdateStatusWithContext.
// Field names and required status follow the transfer.updateStatus request table
// in docs/namecheap-api-v2.md (lines 743-744).
type DomainsTransferUpdateStatusArgs struct {
	// TransferID is the unique Transfer ID to update. Required.
	TransferID int
	// Resubmit resubmits the transfer order (after releasing the registry lock)
	// when true. It is serialized as the doc-mandated string "true"/"false".
	Resubmit bool
}

// UpdateStatusWithContext updates the status of a transfer, typically to
// resubmit it after the registry lock has been released. It is idempotent (a
// repeated resubmit of the same transfer is harmless) and retries on transient
// failures.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-transfer/update-status/
func (dts *DomainsTransferService) UpdateStatusWithContext(ctx context.Context, args *DomainsTransferUpdateStatusArgs) (*DomainsTransferUpdateStatusCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if args.TransferID < 1 {
		return nil, &InvalidArgumentsError{Fields: []string{"TransferID"}, Reason: "TransferID must be a positive integer"}
	}

	params := map[string]string{
		"Command":    "namecheap.domains.transfer.updateStatus",
		"TransferID": strconv.Itoa(args.TransferID),
		"Resubmit":   strconv.FormatBool(args.Resubmit),
	}

	var response DomainsTransferUpdateStatusResponse
	_, err := dts.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
