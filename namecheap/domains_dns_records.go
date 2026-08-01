package namecheap

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// defaultTTL is the TTL Namecheap applies to a host record that is created
// without an explicit TTL (its "automatic" value, 1799 seconds). NormalizeRecord
// treats a nil or zero TTL as this value so a record left on automatic compares
// equal to the value the getHosts API echoes back for it.
//
// Note: the DomainsDNSHostRecord.TTL field comment historically cites 1800; 1799
// is what the API actually returns for an automatic record and is the value
// issue #119 specifies for normalization, so it is used here.
const defaultTTL = 1799

// RecordSelector selects host records for DeleteRecordsWithContext and
// UpsertRecordsWithContext (and for building DeleteOp/UpsertOp).
//
// A record matches when every non-nil field equals the corresponding field of
// the record after normalization: HostName and RecordType are compared
// case-insensitively, a single trailing dot on Address is ignored, and MXPref is
// compared exactly (a nil MXPref on the record never matches a non-nil selector
// MXPref). See NormalizeRecord for the exact rules.
//
// A selector with no fields set and MatchAll false is invalid and is rejected
// before any HTTP call, to refuse an accidental mass-delete. Set MatchAll to true
// to intentionally select every record; it is the only way to select all.
type RecordSelector struct {
	// HostName, when non-nil, matches records whose (normalized) host equals it.
	HostName *string
	// RecordType, when non-nil, matches records whose (normalized) type equals it.
	RecordType *string
	// Address, when non-nil, matches records whose (normalized) address equals it.
	Address *string
	// MXPref, when non-nil, matches records whose MXPref equals it.
	MXPref *uint8
	// MatchAll, when true, matches every record regardless of the other fields.
	MatchAll bool
}

// isEmpty reports whether the selector matches on nothing, which is treated as
// an invalid mass-delete rather than an intentional wipe (use MatchAll for that).
func (s RecordSelector) isEmpty() bool {
	return !s.MatchAll && s.HostName == nil && s.RecordType == nil && s.Address == nil && s.MXPref == nil
}

// matches reports whether r satisfies the selector. Every non-nil selector field
// must equal the corresponding normalized field of r.
func (s RecordSelector) matches(r DomainsDNSHostRecord) bool {
	if s.MatchAll {
		return true
	}
	n := NormalizeRecord(r)
	if s.HostName != nil && normHostName(*s.HostName) != *n.HostName {
		return false
	}
	if s.RecordType != nil && normRecordType(*s.RecordType) != *n.RecordType {
		return false
	}
	if s.Address != nil && normAddress(*s.Address) != *n.Address {
		return false
	}
	if s.MXPref != nil && (n.MXPref == nil || *n.MXPref != *s.MXPref) {
		return false
	}
	return true
}

// RecordDiff is the result of planning a set of record operations against the
// current zone: Add lists the records that would be created, Remove the records
// that would be deleted, and Keep the existing records left untouched. The
// slices are sorted deterministically by normalized identity.
type RecordDiff struct {
	Add    []DomainsDNSHostRecord
	Remove []DomainsDNSHostRecord
	Keep   []DomainsDNSHostRecord
	// EmailType is the zone's current EmailType, as read from getHosts.
	EmailType string
	// RequiredEmailType is the EmailType this change needs the zone to move to,
	// or "" when the current one already works — so a non-empty value means
	// "apply this with WithEmailType(RequiredEmailType)".
	//
	// The requirement runs in both directions, because the SDK's setHosts
	// validation ties mail records to the zone type: adding the first MX record
	// requires moving up to MX, and removing the last one requires moving down to
	// NONE. Ordinary A/CNAME/TXT changes require nothing and leave this empty.
	RequiredEmailType string
	// Satisfiable is false when no EmailType can accept the resulting record set
	// at all — a set holding both MX and MXE records, or more than one MXE — so
	// an empty RequiredEmailType is not mistaken for "nothing to do". Applying
	// such a diff fails whatever EmailType is passed.
	Satisfiable bool
}

// String renders the diff as a stable, human-readable summary.
func (d RecordDiff) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "RecordDiff: +%d -%d =%d", len(d.Add), len(d.Remove), len(d.Keep))
	if d.RequiredEmailType != "" && d.RequiredEmailType != d.EmailType {
		fmt.Fprintf(&b, "\n  EmailType: %s -> %s", d.EmailType, d.RequiredEmailType)
	}
	for _, r := range d.Add {
		fmt.Fprintf(&b, "\n  + %s", recordString(r))
	}
	for _, r := range d.Remove {
		fmt.Fprintf(&b, "\n  - %s", recordString(r))
	}
	for _, r := range d.Keep {
		fmt.Fprintf(&b, "\n    %s", recordString(r))
	}
	return b.String()
}

// RecordChangeResult reports the outcome of a record-level mutation.
type RecordChangeResult struct {
	// Added, Removed and Kept are the counts of records created, deleted and left
	// untouched relative to the zone as it was read.
	Added   int
	Removed int
	Kept    int
	// Records is the full record set written to the zone (its new state).
	Records []DomainsDNSHostRecord
	// Response is the raw setHosts command response for the successful write.
	Response *DomainsDNSSetHostsCommandResponse
}

// RecordOption configures a record-level mutation.
type RecordOption func(*recordOptions)

type recordOptions struct {
	maxAttempts int
	// emailType, when non-nil, replaces the zone EmailType read from getHosts
	// instead of preserving it.
	emailType *string
}

// WithRetryOnConflict enables automatic retry of the whole
// read-modify-write-verify cycle when a concurrent modification is detected, for
// up to maxAttempts total attempts (values below 1 are treated as 1). With no
// option the mutation makes a single attempt and returns ErrConcurrentModification
// on conflict.
func WithRetryOnConflict(maxAttempts int) RecordOption {
	return func(o *recordOptions) {
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		o.maxAttempts = maxAttempts
	}
}

// WithEmailType sets the zone EmailType to write with this mutation instead of
// preserving the value read from getHosts.
//
// It exists because the SDK's setHosts validation couples mail records to the
// zone EmailType: an MX record is rejected unless the zone is EmailType=MX, an
// MXE record unless it is MXE, and an MX zone must keep at least one MX record.
// Preserving the existing value — the default, and the right behaviour for
// ordinary A/CNAME/TXT changes — therefore makes the mail lifecycle unreachable:
// the first MX record cannot be added to a NONE zone, and the last one cannot be
// removed from an MX zone.
//
//	// Add the first MX record to a zone that has no mail routing yet.
//	AddRecordsWithContext(ctx, domain, mx, WithEmailType(EmailTypeMX))
//
//	// Remove the last MX record and turn mail routing off in the same write.
//	DeleteRecordsWithContext(ctx, domain, sel, WithEmailType(EmailTypeNone))
//
// Use it only when the mutation is meant to change mail routing. On a zone whose
// mail is managed elsewhere — EmailTypeForward alongside setEmailForwarding —
// passing an EmailType silently reroutes that mail, so leave the option off and
// let the value be preserved.
//
// Known limitation on EmailTypePrivate (OX) and EmailTypeGmail zones: those
// carry their provider's MX records in the host list while the zone type stays
// OX/GMAIL, a combination the SDK's setHosts validation rejects. The record
// helpers therefore cannot mutate such a zone at all — not even to change an
// unrelated A record — and the resulting error suggests WithEmailType(MX), which
// would switch the domain off the hosted mailbox. Do not follow that suggestion
// on an OX/GMAIL zone; use GetHosts/SetHosts directly until the validation rule
// is verified against the live API.
//
// The resulting record set is validated against the requested EmailType before
// anything is written; see PlanWithContext to preview the transition.
func WithEmailType(emailType string) RecordOption {
	return func(o *recordOptions) {
		o.emailType = &emailType
	}
}

func buildRecordOptions(opts []RecordOption) recordOptions {
	o := recordOptions{maxAttempts: 1}
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

type recordOpKind int

const (
	recordOpAdd recordOpKind = iota
	recordOpDelete
	recordOpUpsert
)

// RecordOp is a single record operation (add, delete or upsert). Build one with
// AddOp, DeleteOp or UpsertOp and pass it to PlanWithContext; the mutating
// helpers build the same values internally so Plan and the writers share one
// diff engine. The zero value is not valid.
type RecordOp struct {
	kind     recordOpKind
	selector RecordSelector
	records  []DomainsDNSHostRecord
}

// AddOp returns an operation that appends records, preserving all existing ones.
func AddOp(records ...DomainsDNSHostRecord) RecordOp {
	return RecordOp{kind: recordOpAdd, records: records}
}

// DeleteOp returns an operation that removes every record matching selector.
func DeleteOp(selector RecordSelector) RecordOp {
	return RecordOp{kind: recordOpDelete, selector: selector}
}

// UpsertOp returns an operation that replaces exactly the records matching
// selector with records (matching records are removed, then records appended).
func UpsertOp(selector RecordSelector, records []DomainsDNSHostRecord) RecordOp {
	return RecordOp{kind: recordOpUpsert, selector: selector, records: records}
}

// RecordFromDetailed converts a DomainsDNSHostRecordDetailed returned by
// GetHosts into a DomainsDNSHostRecord accepted by SetHosts, so a zone can be
// read, modified and written back without losing settable fields.
//
// Field mapping:
//
//	Detailed.Name    -> HostName
//	Detailed.Type    -> RecordType
//	Detailed.Address -> Address
//	Detailed.TTL     -> TTL
//	Detailed.MXPref  -> MXPref (int -> uint8; clamped to 0..255; nil stays nil)
//
// The following read-only, server-managed fields are intentionally dropped
// because setHosts has no input for them and the server regenerates them on
// every write: HostId, AssociatedAppTitle, FriendlyName, IsActive and
// IsDDNSEnabled. Carrying them would be meaningless (setHosts ignores them). The
// knownFields exhaustiveness test asserts every field of the detailed struct is
// either mapped here or consciously dropped, so a new API field fails the test
// instead of being silently lost.
func RecordFromDetailed(d DomainsDNSHostRecordDetailed) DomainsDNSHostRecord {
	return DomainsDNSHostRecord{
		HostName:   d.Name,
		RecordType: d.Type,
		Address:    d.Address,
		TTL:        d.TTL,
		MXPref:     mxPrefFromInt(d.MXPref),
	}
}

// mxPrefFromInt converts a getHosts MXPref (*int) to a setHosts MXPref (*uint8),
// clamping to the valid 0..255 range. A nil input stays nil.
func mxPrefFromInt(p *int) *uint8 {
	if p == nil {
		return nil
	}
	v := *p
	switch {
	case v < 0:
		v = 0
	case v > 255:
		v = 255
	}
	out := uint8(v)
	return &out
}

// normHostName canonicalizes a hostname: trimmed, lower-cased, and an empty host
// mapped to "@" (the apex), matching how the API denotes the zone root.
func normHostName(s string) string {
	h := strings.ToLower(strings.TrimSpace(s))
	if h == "" {
		return "@"
	}
	return h
}

// normRecordType canonicalizes a record type: trimmed and upper-cased.
func normRecordType(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// normAddress canonicalizes an address by removing a single trailing dot. The
// dot is DNS-insignificant for the hostname targets the API echoes back
// (CNAME/MX/ALIAS/NS) and the server may add or drop it, so trimming it keeps a
// read-modify-write round-trip stable. Case is preserved because TXT values are
// case-sensitive.
func normAddress(s string) string {
	return strings.TrimSuffix(s, ".")
}

// NormalizeRecord returns a copy of r in canonical form so two records that mean
// the same thing compare equal (see RecordsEqual). It does not mutate r or the
// values its pointers reference. The normalizations are:
//
//   - HostName: trimmed, lower-cased; an empty or nil host becomes "@" (apex).
//   - RecordType: trimmed, upper-cased.
//   - Address: a single trailing dot is removed (case preserved for TXT).
//   - TTL: a nil or zero TTL becomes the Namecheap automatic default (1799).
//   - MXPref: copied unchanged (nil stays nil).
func NormalizeRecord(r DomainsDNSHostRecord) DomainsDNSHostRecord {
	host := "@"
	if r.HostName != nil {
		host = normHostName(*r.HostName)
	}

	recordType := ""
	if r.RecordType != nil {
		recordType = normRecordType(*r.RecordType)
	}

	address := ""
	if r.Address != nil {
		address = normAddress(*r.Address)
	}

	ttl := defaultTTL
	if r.TTL != nil && *r.TTL != 0 {
		ttl = *r.TTL
	}

	out := DomainsDNSHostRecord{
		HostName:   &host,
		RecordType: &recordType,
		Address:    &address,
		TTL:        &ttl,
	}
	if r.MXPref != nil {
		mx := *r.MXPref
		out.MXPref = &mx
	}
	return out
}

// RecordsEqual reports whether a and b denote the same host record, comparing
// them in normalized form (see NormalizeRecord). It is symmetric and consistent
// with NormalizeRecord, and is the equality used by the diff engine and by the
// post-write verification.
func RecordsEqual(a, b DomainsDNSHostRecord) bool {
	return recordKey(a) == recordKey(b)
}

// recordKey builds a collision-resistant canonical key for a normalized record,
// used for equality and multiset comparisons. It includes every settable field
// (host, type, address, MXPref, TTL) so a change to any of them is detected.
func recordKey(r DomainsDNSHostRecord) string {
	n := NormalizeRecord(r)
	mx := ""
	if n.MXPref != nil {
		mx = strconv.Itoa(int(*n.MXPref))
	}
	return strings.Join([]string{*n.HostName, *n.RecordType, *n.Address, mx, strconv.Itoa(*n.TTL)}, "\x00")
}

// recordString formats a record for diagnostics, nil-safely.
func recordString(r DomainsDNSHostRecord) string {
	return fmt.Sprintf("%s %s %s (mxpref=%v ttl=%v)", derefStr(r.HostName), derefStr(r.RecordType), derefStr(r.Address), deref(r.MXPref), deref(r.TTL))
}

// derefStr returns the string a non-nil pointer references, or "" for nil.
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// validateOps rejects delete/upsert operations carrying an empty selector before
// any HTTP request, guarding against an accidental mass-delete.
func validateOps(ops []RecordOp) error {
	for _, op := range ops {
		if op.kind == recordOpDelete || op.kind == recordOpUpsert {
			if op.selector.isEmpty() {
				return &InvalidArgumentsError{
					Fields: []string{"RecordSelector"},
					Reason: "empty selector would match every record; set at least one field, or MatchAll to intentionally select all",
				}
			}
		}
	}
	return nil
}

// applyOps returns the record set produced by applying ops in order to current.
// It preserves ordering (surviving records keep their relative order, appended
// records follow) so the written zone is deterministic.
func applyOps(current []DomainsDNSHostRecord, ops []RecordOp) []DomainsDNSHostRecord {
	working := slices.Clone(current)
	for _, op := range ops {
		switch op.kind {
		case recordOpAdd:
			working = append(working, op.records...)
		case recordOpDelete:
			working = filterOut(working, op.selector)
		case recordOpUpsert:
			working = append(filterOut(working, op.selector), op.records...)
		}
	}
	return working
}

// filterOut returns the records that do not match selector, in original order.
func filterOut(records []DomainsDNSHostRecord, selector RecordSelector) []DomainsDNSHostRecord {
	kept := make([]DomainsDNSHostRecord, 0, len(records))
	for _, r := range records {
		if !selector.matches(r) {
			kept = append(kept, r)
		}
	}
	return kept
}

// computeDiff applies ops to current and returns both the resulting diff and the
// final record set to be written.
func computeDiff(current []DomainsDNSHostRecord, ops []RecordOp) (RecordDiff, []DomainsDNSHostRecord) {
	final := applyOps(current, ops)
	return diffSets(current, final), final
}

// diffSets compares current and final as multisets keyed by normalized identity:
// records in both are Keep, records only in current are Remove, and records only
// in final are Add. The result slices are sorted for determinism.
func diffSets(current, final []DomainsDNSHostRecord) RecordDiff {
	curByKey := groupByKey(current)
	finalByKey := groupByKey(final)

	diff := RecordDiff{
		Add:    make([]DomainsDNSHostRecord, 0, len(final)),
		Remove: make([]DomainsDNSHostRecord, 0, len(current)),
		Keep:   make([]DomainsDNSHostRecord, 0, len(current)),
	}

	for key, curs := range curByKey {
		keep := min(len(curs), len(finalByKey[key]))
		diff.Keep = append(diff.Keep, curs[:keep]...)
		diff.Remove = append(diff.Remove, curs[keep:]...)
	}
	for key, finals := range finalByKey {
		curN := len(curByKey[key])
		if len(finals) > curN {
			diff.Add = append(diff.Add, finals[curN:]...)
		}
	}

	sortRecords(diff.Add)
	sortRecords(diff.Remove)
	sortRecords(diff.Keep)
	return diff
}

// groupByKey buckets records by their normalized identity key.
func groupByKey(records []DomainsDNSHostRecord) map[string][]DomainsDNSHostRecord {
	m := make(map[string][]DomainsDNSHostRecord, len(records))
	for _, r := range records {
		key := recordKey(r)
		m[key] = append(m[key], r)
	}
	return m
}

// sortRecords orders records deterministically by their normalized identity key.
func sortRecords(records []DomainsDNSHostRecord) {
	slices.SortFunc(records, func(a, b DomainsDNSHostRecord) int {
		return strings.Compare(recordKey(a), recordKey(b))
	})
}

// recordSetsEqual reports whether a and b contain the same records as multisets
// (order-independent), using normalized identity.
func recordSetsEqual(a, b []DomainsDNSHostRecord) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, r := range a {
		counts[recordKey(r)]++
	}
	for _, r := range b {
		counts[recordKey(r)]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}

// requiredEmailType reports the EmailType the zone must carry for records to be
// writable, given the EmailType it currently has. It returns "" when the current
// value already works, and false when no EmailType can accept the set.
//
// The SDK's setHosts validation accepts an MX record only on an MX zone and an
// MXE record only on an MXE zone, and additionally requires an MX zone to keep
// at least one MX record and an MXE zone exactly one. So the requirement runs in
// both directions: adding the first MX record demands a move up to MX, and
// removing the last one demands a move down to NONE.
func requiredEmailType(records []DomainsDNSHostRecord, current string) (string, bool) {
	var mxCount, mxeCount int
	for _, r := range records {
		switch normRecordType(derefStr(r.RecordType)) {
		case RecordTypeMX:
			mxCount++
		case RecordTypeMXE:
			mxeCount++
		}
	}

	switch {
	case mxCount > 0 && mxeCount > 0:
		// No single EmailType admits both.
		return "", false
	case mxCount > 0:
		return transitionTo(EmailTypeMX, current), true
	case mxeCount != 1 && current == EmailTypeMXE:
		// An MXE zone requires exactly one MXE record; without it, mail routing
		// has to come down.
		return EmailTypeNone, true
	case mxeCount == 1:
		return transitionTo(EmailTypeMXE, current), true
	case mxeCount > 1:
		return "", false
	case current == EmailTypeMX:
		// The set has no MX record left, which an MX zone cannot be.
		return EmailTypeNone, true
	default:
		return "", true
	}
}

// transitionTo returns want when it differs from current, and "" when the zone
// is already there — so callers can treat a non-empty value as "you must pass
// WithEmailType".
func transitionTo(want, current string) string {
	if want == current {
		return ""
	}
	return want
}

// validateEmailType checks the resulting record set against the EmailType the
// write will carry, before anything is written.
//
// These are the SDK's client-side setHosts rules (see
// validateDomainsDNSSetHostsArgs), which SetHosts would enforce a moment later
// anyway — but its message cannot tell a record-level caller what to do about
// it. These errors name WithEmailType, because that is the fix. They are
// *InvalidArgumentsError so a caller can tell "your request is invalid, fail
// fast" from "the API call failed, maybe retry".
func validateEmailType(records []DomainsDNSHostRecord, emailType string, explicit bool) error {
	if explicit && !isValidEmailType(emailType) {
		return &InvalidArgumentsError{
			Fields: []string{"EmailType"},
			Reason: fmt.Sprintf("WithEmailType(%q) is not a valid EmailType; allowed values are %s",
				emailType, strings.Join(AllowedEmailTypeValues, ", ")),
		}
	}

	var mxCount, mxeCount int
	for _, r := range records {
		switch normRecordType(derefStr(r.RecordType)) {
		case RecordTypeMX:
			mxCount++
		case RecordTypeMXE:
			mxeCount++
		}
	}

	invalid := func(format string, args ...any) error {
		return &InvalidArgumentsError{Fields: []string{"EmailType"}, Reason: fmt.Sprintf(format, args...)}
	}

	if mxCount > 0 && mxeCount > 0 {
		return invalid("record set contains both MX and MXE records, which no EmailType accepts: "+
			"MX records need EmailType=%s and MXE records need EmailType=%s", EmailTypeMX, EmailTypeMXE)
	}

	source := "the zone's current EmailType"
	if explicit {
		source = "the EmailType passed to WithEmailType"
	}

	switch {
	case mxCount > 0 && emailType != EmailTypeMX:
		return invalid("the resulting record set has %d MX record(s), which the SDK's setHosts validation "+
			"accepts only on an EmailType=%s zone, but %s is %q: pass WithEmailType(EmailTypeMX)",
			mxCount, EmailTypeMX, source, emailType)
	case mxeCount > 0 && emailType != EmailTypeMXE:
		return invalid("the resulting record set has %d MXE record(s), which the SDK's setHosts validation "+
			"accepts only on an EmailType=%s zone, but %s is %q: pass WithEmailType(EmailTypeMXE)",
			mxeCount, EmailTypeMXE, source, emailType)
	case emailType == EmailTypeMX && mxCount == 0:
		return invalid("%s is %s, which the SDK's setHosts validation requires at least one MX record for, "+
			"but the resulting record set has none: pass WithEmailType(EmailTypeNone) to turn mail routing "+
			"off in the same write", source, EmailTypeMX)
	case emailType == EmailTypeMXE && mxeCount != 1:
		return invalid("%s is %s, which the SDK's setHosts validation requires exactly one MXE record for, "+
			"but the resulting record set has %d", source, EmailTypeMXE, mxeCount)
	}
	return nil
}

// recordsFromResult extracts the settable record set and the zone EmailType from
// a getHosts response. A nil or empty EmailType defaults to EmailTypeNone so a
// partial update never silently clears email routing.
func recordsFromResult(resp *DomainsDNSGetHostsCommandResponse) ([]DomainsDNSHostRecord, string) {
	emailType := EmailTypeNone
	if resp == nil || resp.DomainDNSGetHostsResult == nil {
		return nil, emailType
	}
	result := resp.DomainDNSGetHostsResult
	if result.EmailType != nil && *result.EmailType != "" {
		emailType = *result.EmailType
	}
	if result.Hosts == nil {
		return nil, emailType
	}
	hosts := *result.Hosts
	records := make([]DomainsDNSHostRecord, 0, len(hosts))
	for _, h := range hosts {
		records = append(records, RecordFromDetailed(h))
	}
	return records, emailType
}

// buildSetHostsArgs assembles the setHosts arguments for a full-zone write,
// always carrying an explicit EmailType: the value read from getHosts unless the
// caller overrode it with WithEmailType.
func buildSetHostsArgs(domain string, records []DomainsDNSHostRecord, emailType string) *DomainsDNSSetHostsArgs {
	return &DomainsDNSSetHostsArgs{
		Domain:    &domain,
		Records:   &records,
		EmailType: &emailType,
	}
}

// AddRecordsWithContext appends records to domain's zone, preserving every
// existing record (and, unless WithEmailType is supplied, the zone EmailType),
// then verifies the result by
// re-reading the zone. It returns ErrConcurrentModification if the zone changed
// under it, unless WithRetryOnConflict is supplied. See the package README on
// the non-transactional setHosts API.
func (dds *DomainsDNSService) AddRecordsWithContext(ctx context.Context, domain string, records []DomainsDNSHostRecord, opts ...RecordOption) (*RecordChangeResult, error) {
	return dds.applyRecordOps(ctx, domain, []RecordOp{AddOp(records...)}, opts)
}

// DeleteRecordsWithContext removes every record matching selector from domain's
// zone, preserving the rest (and, unless WithEmailType is supplied, the zone
// EmailType), then verifies the result.
// An empty selector is rejected before any request; use
// RecordSelector{MatchAll: true} to intentionally delete all records. It returns
// ErrConcurrentModification on a detected race unless WithRetryOnConflict is
// supplied.
func (dds *DomainsDNSService) DeleteRecordsWithContext(ctx context.Context, domain string, selector RecordSelector, opts ...RecordOption) (*RecordChangeResult, error) {
	return dds.applyRecordOps(ctx, domain, []RecordOp{DeleteOp(selector)}, opts)
}

// UpsertRecordsWithContext replaces exactly the records matching selector with
// records, preserving all other records (and, unless WithEmailType is supplied,
// the zone EmailType), then verifies the result. An empty selector is rejected before any request. It returns
// ErrConcurrentModification on a detected race unless WithRetryOnConflict is
// supplied.
func (dds *DomainsDNSService) UpsertRecordsWithContext(ctx context.Context, domain string, selector RecordSelector, records []DomainsDNSHostRecord, opts ...RecordOption) (*RecordChangeResult, error) {
	return dds.applyRecordOps(ctx, domain, []RecordOp{UpsertOp(selector, records)}, opts)
}

// PlanWithContext computes the add/remove/keep diff that ops would produce
// against domain's current zone, without writing anything: it performs a single
// getHosts and zero setHosts calls. Use it to preview a change before applying it
// with AddRecords/DeleteRecords/UpsertRecords.
func (dds *DomainsDNSService) PlanWithContext(ctx context.Context, domain string, ops ...RecordOp) (RecordDiff, error) {
	if err := validateOps(ops); err != nil {
		return RecordDiff{}, err
	}
	getResp, err := dds.GetHostsWithContext(ctx, domain)
	if err != nil {
		return RecordDiff{}, err
	}
	current, emailType := recordsFromResult(getResp)
	diff, final := computeDiff(current, ops)
	diff.EmailType = emailType
	required, satisfiable := requiredEmailType(final, emailType)
	diff.RequiredEmailType = required
	diff.Satisfiable = satisfiable
	return diff, nil
}

// applyRecordOps runs the read-modify-write-verify cycle for a single logical
// change (built from ops) against domain, honoring the retry-on-conflict option.
// Selectors are validated once, before any HTTP call.
func (dds *DomainsDNSService) applyRecordOps(ctx context.Context, domain string, ops []RecordOp, opts []RecordOption) (*RecordChangeResult, error) {
	if err := validateOps(ops); err != nil {
		return nil, err
	}
	cfg := buildRecordOptions(opts)

	var lastErr error
	for attempt := 0; attempt < cfg.maxAttempts; attempt++ {
		result, conflict, err := dds.attemptRecordOps(ctx, domain, ops, cfg)
		if err != nil {
			return nil, err
		}
		if !conflict {
			return result, nil
		}
		lastErr = ErrConcurrentModification
	}
	return nil, lastErr
}

// attemptRecordOps performs one read-modify-write-verify cycle. It reports a
// conflict (second return value) when the verifying re-read does not match the
// record set it intended to write; a conflict is not an error so the caller can
// decide whether to retry.
func (dds *DomainsDNSService) attemptRecordOps(ctx context.Context, domain string, ops []RecordOp, cfg recordOptions) (*RecordChangeResult, bool, error) {
	getResp, err := dds.GetHostsWithContext(ctx, domain)
	if err != nil {
		return nil, false, err
	}
	current, emailType := recordsFromResult(getResp)

	diff, final := computeDiff(current, ops)

	// WithEmailType replaces the preserved value; without it the zone keeps the
	// EmailType it already had.
	explicitEmailType := cfg.emailType != nil
	if explicitEmailType {
		emailType = *cfg.emailType
	}
	if err := validateEmailType(final, emailType, explicitEmailType); err != nil {
		return nil, false, err
	}

	setResp, err := dds.SetHostsWithContext(ctx, buildSetHostsArgs(domain, final, emailType))
	if err != nil {
		return nil, false, err
	}

	verifyResp, err := dds.GetHostsWithContext(ctx, domain)
	if err != nil {
		return nil, false, err
	}
	got, gotEmailType := recordsFromResult(verifyResp)
	// The EmailType is part of what this write set, so a concurrent writer that
	// reverted it has to count as a conflict — otherwise WithEmailType's own
	// effect would sit outside the guarantee this layer advertises, and the
	// caller would be told the mail transition succeeded when it did not.
	if !recordSetsEqual(got, final) || gotEmailType != emailType {
		return nil, true, nil
	}

	return &RecordChangeResult{
		Added:    len(diff.Add),
		Removed:  len(diff.Remove),
		Kept:     len(diff.Keep),
		Records:  final,
		Response: setResp,
	}, false, nil
}
