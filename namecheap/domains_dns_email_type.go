package namecheap

import (
	"fmt"
	"slices"
	"strings"
)

// EmailType values are a zone's mail mode, sent as the EmailType parameter of
// setHosts and read back from getHosts.
//
// These are the mail-mode rules of setHosts, verified against the live API
// (sandbox and production, 2026-10-08) and pinned by TestAcc_EmailType. The
// rest of the package refers to them here instead of restating them:
//
//   - Omitting EmailType keeps the zone's current mode. The API reference marks
//     the parameter "Required: Yes"; it is not.
//   - MX records are kept only under EmailType=MX. Under any other mode
//     Namecheap silently discards them.
//   - MXE records are kept under EmailType=MXE and MX. Under the other modes
//     the behaviour is unverified, so the SDK does not send them there.
//   - No MX or MXE record counts are enforced: an MX zone may hold no MX
//     record, an MXE zone none or several MXE records.
//   - An unknown EmailType answers Status="OK" with IsSuccess="false" and
//     leaves the zone unchanged.
//
// The AssociatedAppTitle tag getHosts puts on a row a preset created changes
// none of this (sandbox and production, 2026-10-28): a tagged MX row is dropped
// under a non-MX mode like any other, and setHosts stores a caller-supplied tag
// verbatim, so the tag is neither protection nor proof of ownership. The SDK
// echoes tagged rows as they are and never sends the tag.
const (
	EmailTypeNone    = "NONE"
	EmailTypeMXE     = "MXE"
	EmailTypeMX      = "MX"
	EmailTypeForward = "FWD"
	EmailTypePrivate = "OX"
	EmailTypeGmail   = "GMAIL"
)

// AllowedEmailTypeValues lists every EmailType value setHosts accepts.
var AllowedEmailTypeValues = []string{EmailTypeNone, EmailTypeMXE, EmailTypeMX, EmailTypeForward, EmailTypePrivate, EmailTypeGmail}

func isValidEmailType(emailType string) bool {
	return slices.Contains(AllowedEmailTypeValues, emailType)
}

// emailTypesKeeping returns the modes under which a record of recordType
// survives a setHosts write: MX only for MX, MXE or MX for MXE. It returns nil
// when every mode keeps the record.
func emailTypesKeeping(recordType string) []string {
	switch normRecordType(recordType) {
	case RecordTypeMX:
		return []string{EmailTypeMX}
	case RecordTypeMXE:
		return []string{EmailTypeMXE, EmailTypeMX}
	}
	return nil
}

// checkMailRecordEmailType returns an error when Namecheap would drop record
// from a setHosts write sent with emailType: an MX record needs EmailType=MX,
// an MXE record MXE or MX. Every other record passes under any EmailType; a
// preset tag on the record makes no difference.
//
// A nil emailType means the parameter is omitted. The zone then keeps its
// current mode, which the caller cannot see from the request alone, so an MX
// or MXE record is reported: the write would keep it only on a zone already in
// the right mode.
func checkMailRecordEmailType(record DomainsDNSHostRecord, emailType *string) error {
	keeping := emailTypesKeeping(derefStr(record.RecordType))
	if keeping == nil || (emailType != nil && slices.Contains(keeping, *emailType)) {
		return nil
	}
	need := strings.Join(keeping, " or ")
	if emailType == nil {
		return fmt.Errorf("RecordType %s needs EmailType=%s; with EmailType omitted Namecheap keeps it only on a zone already in that mode",
			derefStr(record.RecordType), need)
	}
	return fmt.Errorf("RecordType %s needs EmailType=%s, not %s", derefStr(record.RecordType), need, *emailType)
}

// deriveEmailType returns the EmailType a write that adds records needs on a
// zone currently in mode current, or "" when the parameter should be omitted so
// the zone keeps its mode. MX rows need MX; MXE rows need MXE, or MX on a zone
// already on MX. Anything else omits the parameter: never NONE, never a
// downgrade, and a preset mode (FWD, OX, GMAIL) is left alone.
//
// records are the records the operations add, not the zone being echoed: rows
// read from the zone never drive the mode, so an unrelated write cannot move a
// zone that somehow holds an MX row off its preset. MX rows live only under
// MX, so with the parameter omitted an MX zone keeps the ones it has.
func deriveEmailType(records []DomainsDNSHostRecord, current string) string {
	hasMXE := false
	for _, record := range records {
		switch normRecordType(derefStr(record.RecordType)) {
		case RecordTypeMX:
			// MX wins over MXE: MX rows survive only under MX, and MXE rows are
			// kept under MX as well.
			return EmailTypeMX
		case RecordTypeMXE:
			hasMXE = true
		}
	}
	if !hasMXE {
		return ""
	}
	if current == EmailTypeMX {
		return EmailTypeMX
	}
	return EmailTypeMXE
}

// emailTypeToSend returns the EmailType parameter for a write of final on a
// zone in mode current, given the records the operations add: the derived
// transition, or, when nothing is derived but final still carries MX or MXE
// rows, the zone's current mode. Omitting the parameter would keep the mode
// just the same, but SetHosts cannot see the zone and checks MX and MXE rows
// against the parameter, so the mode is spelled out for it. "" omits the
// parameter.
func emailTypeToSend(added, final []DomainsDNSHostRecord, current string) string {
	if derived := deriveEmailType(added, current); derived != "" {
		return derived
	}
	for _, record := range final {
		if emailTypesKeeping(derefStr(record.RecordType)) != nil {
			return current
		}
	}
	return ""
}
