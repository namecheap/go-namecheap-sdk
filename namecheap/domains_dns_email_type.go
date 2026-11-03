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
// Storing is not serving: Namecheap publishes an MXE record, as an MX and an A
// record for its IP, only while the zone is on MXE. Under MX the row is kept
// but not published, so mail to the domain stops without an error. This side
// comes from Namecheap's DNS export (sandbox, 2026-10-30), not from setHosts.
//
// The record helpers derive the mode from those rules in one sentence: a write
// that adds MX or MXE records puts the zone on MX if any MX row remains in the
// written set, otherwise on MXE, and a write that adds neither leaves the mode
// alone (see deriveEmailType for the decision).
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
		return fmt.Errorf("RecordType %s needs EmailType=%s; "+
			"with EmailType omitted Namecheap keeps it only on a zone already in that mode",
			derefStr(record.RecordType), need)
	}
	return fmt.Errorf("RecordType %s needs EmailType=%s, not %s", derefStr(record.RecordType), need, *emailType)
}

// deriveEmailType returns the EmailType a write needs, given the records its
// operations add and final, the record set it writes; "" omits the parameter so
// the zone keeps its mode.
//
// Decision: when the write adds MX or MXE records, the mode follows the mail
// rows in final: MX if any MX row remains (MX rows live only under MX, and MXE
// rows are kept under MX too, so MX wins when both are there), otherwise MXE if
// any MXE row remains. The zone's current mode is not read. It used to be, as a
// stand-in for "MX rows still exist", and that kept an MX zone on MX when the
// same write replaced its last MX row with an MXE row: the MXE row was stored
// but, outside MXE, not served.
//
// A write that adds no MX or MXE record omits the parameter: never NONE, never a
// downgrade. Deletes add nothing, so removing the last MX row leaves an MX zone
// on MX, and rows echoed from the zone never drive the mode, so an unrelated
// write cannot move a zone off its preset. The one transition this adds over a
// first mail row is MX -> MXE, when the caller adds an MXE row and no MX row
// survives. Moving off a preset still needs WithEmailType (checkPresetSwitch).
func deriveEmailType(added, final []DomainsDNSHostRecord) string {
	if !slices.ContainsFunc(added, isMailRecord) {
		return ""
	}
	hasMXE := false
	for _, record := range final {
		switch normRecordType(derefStr(record.RecordType)) {
		case RecordTypeMX:
			return EmailTypeMX
		case RecordTypeMXE:
			hasMXE = true
		}
	}
	if hasMXE {
		return EmailTypeMXE
	}
	return ""
}

// isMailRecord reports whether record is an MX or MXE record, whose survival
// depends on the zone's EmailType.
func isMailRecord(record DomainsDNSHostRecord) bool {
	return emailTypesKeeping(derefStr(record.RecordType)) != nil
}

// emailTypeToSend returns the EmailType parameter for a write of final on a
// zone in mode current, given the records the operations add: the derived
// transition, or, when nothing is derived but final still carries MX or MXE
// rows, the zone's current mode. Omitting the parameter would keep the mode
// just the same, but SetHosts cannot see the zone and checks MX and MXE rows
// against the parameter, so the mode is spelled out for it. "" omits the
// parameter.
func emailTypeToSend(added, final []DomainsDNSHostRecord, current string) string {
	if derived := deriveEmailType(added, final); derived != "" {
		return derived
	}
	if slices.ContainsFunc(final, isMailRecord) {
		return current
	}
	return ""
}

// isRecordDrivenEmailType reports whether emailType is a mode the record
// helpers may move on their own: NONE, MX or MXE, whose mail is the zone's own
// MX and MXE rows. Every other mode, FWD, OX, GMAIL and any mode the SDK does
// not know yet, is treated as a preset whose mail Namecheap serves itself.
func isRecordDrivenEmailType(emailType string) bool {
	return emailType == EmailTypeNone || emailType == EmailTypeMX || emailType == EmailTypeMXE
}

// checkPresetSwitch returns an *InvalidArgumentsError when a write sending
// EmailType send would move a zone off the preset current. The preset decides
// where the domain's mail goes, and the MX or MXE records that need the switch
// would be dropped under it, so the helpers make that switch only when the
// caller asks for it with WithEmailType.
//
// The check lists the modes the helpers may leave (NONE, MX, MXE) rather than
// the presets, so a mode Namecheap adds later is never switched off silently.
func checkPresetSwitch(current, send string) error {
	if send == "" || send == current || isRecordDrivenEmailType(current) {
		return nil
	}
	return &InvalidArgumentsError{Fields: []string{"EmailType"}, Reason: fmt.Sprintf(
		"the zone is on the %s mail preset and the records added need EmailType=%s, "+
			"which replaces the preset's mail routing; pass WithEmailType(%q) to switch it",
		current, send, send)}
}
