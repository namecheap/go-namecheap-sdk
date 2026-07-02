package namecheap

import (
	"context"
	"encoding/xml"
)

// LockAction is the explicit registrar-lock action passed to
// SetRegistrarLockWithContext. Using a dedicated type (rather than a bare bool)
// keeps call sites self-documenting and maps directly onto the API's LOCK/UNLOCK
// values.
type LockAction string

const (
	// RegistrarLock locks the domain at the registrar (API value "LOCK").
	RegistrarLock LockAction = "LOCK"
	// RegistrarUnlock unlocks the domain at the registrar (API value "UNLOCK").
	RegistrarUnlock LockAction = "UNLOCK"
)

// valid reports whether the action is one of the two supported values.
func (a LockAction) valid() bool {
	return a == RegistrarLock || a == RegistrarUnlock
}

// DomainsSetRegistrarLockResponse is the raw envelope for
// namecheap.domains.setRegistrarLock.
type DomainsSetRegistrarLockResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsSetRegistrarLockCommandResponse `xml:"CommandResponse"`
}

// DomainsSetRegistrarLockCommandResponse wraps the setRegistrarLock result.
type DomainsSetRegistrarLockCommandResponse struct {
	DomainSetRegistrarLockResult *DomainsSetRegistrarLockResult `xml:"DomainSetRegistrarLockResult"`
}

// DomainsSetRegistrarLockResult is the outcome of a setRegistrarLock call.
// Fields follow the response table in docs/namecheap-api-v2.md (lines 359-362).
type DomainsSetRegistrarLockResult struct {
	// Domain is the domain whose lock status was set.
	Domain *string `xml:"Domain,attr"`
	// IsSuccess indicates whether the registrar lock was set successfully.
	IsSuccess *bool `xml:"IsSuccess,attr"`
}

// SetRegistrarLockWithContext sets the registrar-lock status of a domain. Pass
// RegistrarLock or RegistrarUnlock for action. It is not charge-bearing, so it
// is treated as idempotent for retry purposes.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains/set-registrar-lock/
func (ds *DomainsService) SetRegistrarLockWithContext(ctx context.Context, domain string, action LockAction) (*DomainsSetRegistrarLockCommandResponse, error) {
	missing := make([]string, 0, 2)
	if domain == "" {
		missing = append(missing, "DomainName")
	}
	if !action.valid() {
		missing = append(missing, "LockAction")
	}
	if len(missing) > 0 {
		reason := ""
		if !action.valid() {
			reason = "LockAction must be RegistrarLock or RegistrarUnlock"
		}
		return nil, &InvalidArgumentsError{Fields: missing, Reason: reason}
	}

	var response DomainsSetRegistrarLockResponse
	params := map[string]string{
		"Command":    "namecheap.domains.setRegistrarLock",
		"DomainName": domain,
		"LockAction": string(action),
	}

	_, err := ds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}
