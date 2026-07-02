package namecheap

import (
	"context"
	"encoding/xml"
	"strconv"
)

// DomainsTransferCreateResponse is the raw envelope for
// namecheap.domains.transfer.create.
type DomainsTransferCreateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsTransferCreateCommandResponse `xml:"CommandResponse"`
}

// DomainsTransferCreateCommandResponse wraps the transfer.create result.
type DomainsTransferCreateCommandResponse struct {
	DomainTransferCreateResult *DomainsTransferCreateResult `xml:"DomainTransferCreateResult"`
}

// DomainsTransferCreateResult is the outcome of an inbound transfer request.
// Fields follow the transfer.create response table in docs/namecheap-api-v2.md
// (lines 698-708): DomainName, TransferID, StatusID, OrderID, TransactionID and
// ChargedAmount.
type DomainsTransferCreateResult struct {
	// DomainName is the domain name being transferred.
	DomainName *string `xml:"DomainName,attr"`
	// TransferID is the unique integer identifying the transfer.
	TransferID *int `xml:"TransferID,attr"`
	// StatusID is the raw numeric status code of the transfer. The API doc does
	// not enumerate these codes, so it is exposed verbatim (see TransferState).
	StatusID *int `xml:"StatusID,attr"`
	// OrderID is the unique integer identifying the order.
	OrderID *int `xml:"OrderID,attr"`
	// TransactionID is the unique integer identifying the transaction.
	TransactionID *int `xml:"TransactionID,attr"`
	// ChargedAmount is the amount charged for the transfer, kept as an exact
	// decimal string (see Amount); money is never modeled as float64.
	ChargedAmount *Amount `xml:"ChargedAmount,attr"`
}

// DomainsTransferCreateArgs are the arguments for CreateWithContext. Field names
// and required/optional status follow the transfer.create request table in
// docs/namecheap-api-v2.md (lines 689-696).
//
// Secret handling. EPPCode is a transfer authorization credential: it is placed
// only in the outbound request parameters and is a member of the observability
// secret-key set, so the request/response hooks and slog integration always
// redact it to "***" (see RequestInfo and redactParams). Do not add ad-hoc
// logging of this value.
type DomainsTransferCreateArgs struct {
	// DomainName is the domain to transfer in. Required.
	DomainName string
	// Years is the number of years to add. Required; the API accepts 1 year only
	// (doc line 692), and this SDK requires an explicit value >= 1 for a
	// charge-bearing call.
	Years int
	// EPPCode is the transfer authorization (EPP/auth) code, required for most
	// TLDs. It is treated as a secret and redacted on every observability surface.
	EPPCode string
	// PromotionCode is an optional promotional (coupon) code.
	PromotionCode string
	// AddFreeWhoisguard, when non-nil, adds (true) or omits (false) free domain
	// privacy. Nil leaves the API default (Yes).
	AddFreeWhoisguard *bool
	// WGenable, when non-nil, enables (true) or disables (false) domain privacy.
	// Nil leaves the API default (No). Sent as the "WGenable" parameter per the
	// doc (line 696).
	WGenable *bool
}

// CreateWithContext initiates an inbound domain transfer to Namecheap.
//
// It is a charge-bearing, non-idempotent call: on an ambiguous transport or
// server-side failure the SDK does NOT retry (a resend could double-charge), so
// it uses the non-idempotent transport path exactly like domains.create. Only
// Namecheap's pre-execution HTTP 405 rate-limit signal is retried; reconcile any
// ambiguous failure via the account order history.
//
// Missing required fields (DomainName, Years, EPPCode) are reported together as
// an *InvalidArgumentsError before any request is sent, so no charge can occur.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-transfer/create/
func (dts *DomainsTransferService) CreateWithContext(ctx context.Context, args *DomainsTransferCreateArgs) (*DomainsTransferCreateCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response DomainsTransferCreateResponse
	// idempotent=false: never retry a charge-bearing call on an ambiguous error.
	_, err := dts.client.doXML(ctx, args.params(), &response, false)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports all missing required fields at once.
func (a *DomainsTransferCreateArgs) validate() error {
	missing := make([]string, 0, 3)
	if a.DomainName == "" {
		missing = append(missing, "DomainName")
	}
	if a.Years < 1 {
		missing = append(missing, "Years")
	}
	if a.EPPCode == "" {
		missing = append(missing, "EPPCode")
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map.
func (a *DomainsTransferCreateArgs) params() map[string]string {
	params := map[string]string{
		"Command":    "namecheap.domains.transfer.create",
		"DomainName": a.DomainName,
		"Years":      strconv.Itoa(a.Years),
		"EPPCode":    a.EPPCode,
	}
	setIfNotEmpty(params, "PromotionCode", a.PromotionCode)
	if a.AddFreeWhoisguard != nil {
		params["AddFreeWhoisguard"] = yesNo(*a.AddFreeWhoisguard)
	}
	if a.WGenable != nil {
		params["WGenable"] = yesNo(*a.WGenable)
	}
	return params
}
