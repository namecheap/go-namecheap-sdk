package namecheap

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// dnsMock is a stateful in-memory Namecheap DNS server for httptest. It behaves
// like the real, non-transactional API: getHosts renders the current zone and
// setHosts fully replaces it. This lets the read-modify-write helpers be
// exercised end to end, with an afterSet hook to simulate a concurrent writer.
type dnsMock struct {
	mu        sync.Mutex
	emailType string
	zone      []DomainsDNSHostRecordDetailed
	getCount  int
	setCount  int
	setParams []url.Values
	nextID    int
	// afterSet, when set, runs after each setHosts has replaced the zone, with the
	// 1-based setHosts call index. It may mutate m.zone to simulate a concurrent
	// modification by another writer.
	afterSet func(m *dnsMock, setIndex int)
}

func newDNSMock(emailType string, zone []DomainsDNSHostRecordDetailed) *dnsMock {
	return &dnsMock{emailType: emailType, zone: zone, nextID: 900000}
}

func (m *dnsMock) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	query, _ := url.ParseQuery(string(body))

	m.mu.Lock()
	defer m.mu.Unlock()

	switch query.Get("Command") {
	case "namecheap.domains.dns.getHosts":
		m.getCount++
		_, _ = w.Write([]byte(m.renderGetHosts()))
	case "namecheap.domains.dns.setHosts":
		m.setCount++
		m.setParams = append(m.setParams, query)
		m.zone = m.parseSetHosts(query)
		if et := query.Get("EmailType"); et != "" {
			m.emailType = et
		}
		if m.afterSet != nil {
			m.afterSet(m, m.setCount)
		}
		_, _ = w.Write([]byte(setHostsOKResponse))
	default:
		_, _ = w.Write([]byte(setHostsOKResponse))
	}
}

func (m *dnsMock) renderGetHosts() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">`)
	b.WriteString(`<Errors /><Warnings />`)
	b.WriteString(`<CommandResponse Type="namecheap.domains.dns.getHosts">`)
	b.WriteString(`<DomainDNSGetHostsResult Domain="domain.net" EmailType="` + xmlAttr(m.emailType) + `" IsUsingOurDNS="true">`)
	for _, h := range m.zone {
		b.WriteString(renderHost(h))
	}
	b.WriteString(`</DomainDNSGetHostsResult></CommandResponse></ApiResponse>`)
	return b.String()
}

func (m *dnsMock) parseSetHosts(q url.Values) []DomainsDNSHostRecordDetailed {
	var zone []DomainsDNSHostRecordDetailed
	for i := 1; ; i++ {
		idx := strconv.Itoa(i)
		recordType := q.Get("RecordType" + idx)
		if recordType == "" {
			break
		}
		m.nextID++
		rec := DomainsDNSHostRecordDetailed{
			HostId:  Int(m.nextID),
			Name:    String(q.Get("HostName" + idx)),
			Type:    String(recordType),
			Address: String(q.Get("Address" + idx)),
		}
		if v := q.Get("TTL" + idx); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				rec.TTL = Int(n)
			}
		}
		if v := q.Get("MXPref" + idx); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				rec.MXPref = Int(n)
			}
		}
		zone = append(zone, rec)
	}
	return zone
}

const setHostsOKResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors /><Warnings />
	<CommandResponse Type="namecheap.domains.dns.setHosts">
		<DomainDNSSetHostsResult Domain="domain.net" IsSuccess="true"><Warnings /></DomainDNSSetHostsResult>
	</CommandResponse>
</ApiResponse>`

func renderHost(h DomainsDNSHostRecordDetailed) string {
	attrs := []string{
		`HostId="` + strconv.Itoa(derefIntZero(h.HostId)) + `"`,
		`Name="` + xmlAttr(derefStr(h.Name)) + `"`,
		`Type="` + xmlAttr(derefStr(h.Type)) + `"`,
		`Address="` + xmlAttr(derefStr(h.Address)) + `"`,
	}
	if h.MXPref != nil {
		attrs = append(attrs, `MXPref="`+strconv.Itoa(*h.MXPref)+`"`)
	}
	if h.TTL != nil {
		attrs = append(attrs, `TTL="`+strconv.Itoa(*h.TTL)+`"`)
	}
	attrs = append(attrs, `AssociatedAppTitle=""`, `FriendlyName=""`, `IsActive="true"`, `IsDDNSEnabled="false"`)
	return "<host " + strings.Join(attrs, " ") + " />"
}

func xmlAttr(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func derefIntZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func mockedClient(m *dnsMock) (*Client, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(m.handle))
	client := setupClient(nil)
	client.BaseURL = server.URL
	return client, server
}

// detailed builds a getHosts-style detailed record. A nil mxpref leaves MXPref
// unset (as the API does for non-MX records).
func detailed(name, recordType, address string, mxpref *int, ttl int) DomainsDNSHostRecordDetailed {
	d := DomainsDNSHostRecordDetailed{
		HostId:             Int(100),
		Name:               String(name),
		Type:               String(recordType),
		Address:            String(address),
		TTL:                Int(ttl),
		AssociatedAppTitle: String(""),
		FriendlyName:       String(""),
		IsActive:           Bool(true),
		IsDDNSEnabled:      Bool(false),
	}
	if mxpref != nil {
		d.MXPref = mxpref
	}
	return d
}

// fixtureAllTypes returns a zone containing every documented record type except
// MXE. MX and MXE cannot coexist under a single EmailType (setHosts validation),
// so this fixture uses EmailType=MX and includes the MX record; MXE is covered
// separately by the Plan fixture (no write) and by TestRecordFromDetailed.
func fixtureAllTypes() []DomainsDNSHostRecordDetailed {
	return []DomainsDNSHostRecordDetailed{
		detailed("@", RecordTypeA, "10.0.0.1", nil, 1800),
		detailed("ipv6", RecordTypeAAAA, "2001:db8::1", nil, 1800),
		detailed("alias", RecordTypeAlias, "target.example.com", nil, 1800),
		detailed("@", RecordTypeCAA, `0 issue "letsencrypt.org"`, nil, 1800),
		detailed("blog", RecordTypeCNAME, "hosting.example.com", nil, 1800),
		detailed("mail", RecordTypeMX, "mailhost.example.com", Int(10), 1800),
		detailed("@", RecordTypeNS, "dns1.example.com", nil, 1800),
		detailed("@", RecordTypeTXT, "v=spf1 include:_spf.example.com ~all", nil, 1800),
		detailed("go", RecordTypeURL, "https://example.com/go", nil, 1800),
		detailed("old", RecordTypeURL301, "https://example.com/new", nil, 1800),
		detailed("frame", RecordTypeFrame, "https://example.com/framed", nil, 1800),
	}
}

// fixtureEveryType extends fixtureAllTypes with an MXE record (12 types). It is
// used only by Plan, which never writes and so never triggers setHosts's MX/MXE
// mutual-exclusion validation.
func fixtureEveryType() []DomainsDNSHostRecordDetailed {
	return append(fixtureAllTypes(), detailed("mxe", RecordTypeMXE, "10.0.0.5", nil, 1800))
}

// requestRecordIndex returns the 1-based index of the record in a setHosts
// request matching host/type/address, or 0 if absent.
func requestRecordIndex(params url.Values, host, recordType, address string) int {
	for i := 1; ; i++ {
		idx := strconv.Itoa(i)
		rt := params.Get("RecordType" + idx)
		if rt == "" {
			return 0
		}
		if params.Get("HostName"+idx) == host && rt == recordType && params.Get("Address"+idx) == address {
			return i
		}
	}
}

func requestHasHost(params url.Values, host, recordType string) bool {
	for i := 1; ; i++ {
		idx := strconv.Itoa(i)
		rt := params.Get("RecordType" + idx)
		if rt == "" {
			return false
		}
		if params.Get("HostName"+idx) == host && rt == recordType {
			return true
		}
	}
}

func assertRequestHasRecord(t *testing.T, params url.Values, host, recordType, address, ttl, mxpref string) {
	t.Helper()
	i := requestRecordIndex(params, host, recordType, address)
	if i == 0 {
		t.Errorf("record %s/%s/%s not found in setHosts request", host, recordType, address)
		return
	}
	idx := strconv.Itoa(i)
	if ttl != "" {
		assert.Equal(t, ttl, params.Get("TTL"+idx), "TTL for %s/%s", host, recordType)
	}
	if mxpref != "" {
		assert.Equal(t, mxpref, params.Get("MXPref"+idx), "MXPref for %s/%s", host, recordType)
	}
}

func zoneHas(m *dnsMock, host, recordType, address string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range m.zone {
		if derefStr(h.Name) == host && derefStr(h.Type) == recordType && derefStr(h.Address) == address {
			return true
		}
	}
	return false
}

func TestRecordFromDetailed(t *testing.T) {
	t.Parallel()

	t.Run("maps settable fields and drops read-only ones", func(t *testing.T) {
		t.Parallel()
		d := DomainsDNSHostRecordDetailed{
			HostId:             Int(123),
			Name:               String("mail"),
			Type:               String(RecordTypeMX),
			Address:            String("mailhost.example.com"),
			MXPref:             Int(10),
			TTL:                Int(1800),
			AssociatedAppTitle: String("app"),
			FriendlyName:       String("friendly"),
			IsActive:           Bool(true),
			IsDDNSEnabled:      Bool(true),
		}
		r := RecordFromDetailed(d)
		assert.Equal(t, "mail", *r.HostName)
		assert.Equal(t, RecordTypeMX, *r.RecordType)
		assert.Equal(t, "mailhost.example.com", *r.Address)
		assert.Equal(t, 1800, *r.TTL)
		assert.Equal(t, uint8(10), *r.MXPref)
	})

	t.Run("nil mxpref stays nil", func(t *testing.T) {
		t.Parallel()
		r := RecordFromDetailed(DomainsDNSHostRecordDetailed{Name: String("@"), Type: String(RecordTypeA), Address: String("1.2.3.4")})
		assert.Nil(t, r.MXPref)
	})

	t.Run("mxpref clamped to upper bound", func(t *testing.T) {
		t.Parallel()
		r := RecordFromDetailed(DomainsDNSHostRecordDetailed{MXPref: Int(9000)})
		assert.Equal(t, uint8(255), *r.MXPref)
	})

	t.Run("mxpref clamped to lower bound", func(t *testing.T) {
		t.Parallel()
		r := RecordFromDetailed(DomainsDNSHostRecordDetailed{MXPref: Int(-5)})
		assert.Equal(t, uint8(0), *r.MXPref)
	})

	t.Run("every documented record type maps its type through", func(t *testing.T) {
		t.Parallel()
		for _, rt := range AllowedRecordTypeValues {
			r := RecordFromDetailed(DomainsDNSHostRecordDetailed{Name: String("@"), Type: String(rt), Address: String("x")})
			assert.Equal(t, rt, *r.RecordType)
		}
	})
}

// TestKnownFieldsGuard fails loudly if a field is added to the getHosts response
// structs without being consciously mapped or dropped in the record mapping.
func TestKnownFieldsGuard(t *testing.T) {
	t.Parallel()

	detailedHandled := map[string]struct{}{
		"HostId": {}, "Name": {}, "Type": {}, "Address": {}, "MXPref": {},
		"TTL": {}, "AssociatedAppTitle": {}, "FriendlyName": {}, "IsActive": {},
		"IsDDNSEnabled": {},
	}
	assertAllExportedFieldsHandled(t, reflect.TypeOf(DomainsDNSHostRecordDetailed{}), detailedHandled)

	resultHandled := map[string]struct{}{
		"Domain": {}, "EmailType": {}, "IsUsingOurDNS": {}, "Hosts": {},
	}
	assertAllExportedFieldsHandled(t, reflect.TypeOf(DomainDNSGetHostsResult{}), resultHandled)
}

func assertAllExportedFieldsHandled(t *testing.T, typ reflect.Type, handled map[string]struct{}) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		if _, ok := handled[f.Name]; !ok {
			t.Errorf("%s.%s is not accounted for in the record mapping; update RecordFromDetailed / recordsFromResult and this guard so the field is either mapped or consciously dropped", typ.Name(), f.Name)
		}
	}
}

func TestAddRecordsFidelity(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeMX, fixtureAllTypes())
	client, server := mockedClient(m)
	defer server.Close()

	res, err := client.DomainsDNS.AddRecordsWithContext(context.Background(), "domain.net",
		[]DomainsDNSHostRecord{
			{HostName: String("new"), RecordType: String(RecordTypeA), Address: String("10.0.0.9"), TTL: Int(1800)},
		})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 1, res.Added)
	assert.Equal(t, 0, res.Removed)
	assert.Equal(t, 11, res.Kept)
	assert.Len(t, res.Records, 12)

	// Exactly one setHosts, and EmailType is preserved in it.
	assert.Len(t, m.setParams, 1)
	assert.Equal(t, EmailTypeMX, m.setParams[0].Get("EmailType"))

	// Unrelated records survive with all settable fields intact.
	assertRequestHasRecord(t, m.setParams[0], "blog", RecordTypeCNAME, "hosting.example.com", "1800", "")
	assertRequestHasRecord(t, m.setParams[0], "mail", RecordTypeMX, "mailhost.example.com", "1800", "10")
	assertRequestHasRecord(t, m.setParams[0], "@", RecordTypeTXT, "v=spf1 include:_spf.example.com ~all", "1800", "")
	assertRequestHasRecord(t, m.setParams[0], "@", RecordTypeCAA, `0 issue "letsencrypt.org"`, "1800", "")
	// The intended change happened.
	assertRequestHasRecord(t, m.setParams[0], "new", RecordTypeA, "10.0.0.9", "1800", "")
	assert.True(t, zoneHas(m, "new", RecordTypeA, "10.0.0.9"))
}

func TestDeleteRecordsFidelity(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeMX, fixtureAllTypes())
	client, server := mockedClient(m)
	defer server.Close()

	res, err := client.DomainsDNS.DeleteRecordsWithContext(context.Background(), "domain.net",
		RecordSelector{HostName: String("blog"), RecordType: String(RecordTypeCNAME)})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 0, res.Added)
	assert.Equal(t, 1, res.Removed)
	assert.Equal(t, 10, res.Kept)
	assert.Len(t, res.Records, 10)

	assert.Equal(t, EmailTypeMX, m.setParams[0].Get("EmailType"))
	// Target gone, unrelated records survive with fields intact.
	assert.False(t, requestHasHost(m.setParams[0], "blog", RecordTypeCNAME))
	assertRequestHasRecord(t, m.setParams[0], "mail", RecordTypeMX, "mailhost.example.com", "1800", "10")
	assertRequestHasRecord(t, m.setParams[0], "@", RecordTypeTXT, "v=spf1 include:_spf.example.com ~all", "1800", "")
	assert.False(t, zoneHas(m, "blog", RecordTypeCNAME, "hosting.example.com"))
}

func TestUpsertRecordsFidelity(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeMX, fixtureAllTypes())
	client, server := mockedClient(m)
	defer server.Close()

	res, err := client.DomainsDNS.UpsertRecordsWithContext(context.Background(), "domain.net",
		RecordSelector{RecordType: String(RecordTypeTXT)},
		[]DomainsDNSHostRecord{
			{HostName: String("@"), RecordType: String(RecordTypeTXT), Address: String("v=spf1 -all"), TTL: Int(1800)},
			{HostName: String("_dmarc"), RecordType: String(RecordTypeTXT), Address: String("v=DMARC1; p=reject"), TTL: Int(1800)},
		})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 2, res.Added)
	assert.Equal(t, 1, res.Removed)
	assert.Equal(t, 10, res.Kept)
	assert.Len(t, res.Records, 12)

	assert.Equal(t, EmailTypeMX, m.setParams[0].Get("EmailType"))
	// Old TXT replaced by the two new TXT records.
	assert.Equal(t, 0, requestRecordIndex(m.setParams[0], "@", RecordTypeTXT, "v=spf1 include:_spf.example.com ~all"))
	assertRequestHasRecord(t, m.setParams[0], "@", RecordTypeTXT, "v=spf1 -all", "1800", "")
	assertRequestHasRecord(t, m.setParams[0], "_dmarc", RecordTypeTXT, "v=DMARC1; p=reject", "1800", "")
	// Untouched MX survives with fields intact.
	assertRequestHasRecord(t, m.setParams[0], "mail", RecordTypeMX, "mailhost.example.com", "1800", "10")
}

func TestConcurrentModificationDetected(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeNone, []DomainsDNSHostRecordDetailed{
		detailed("@", RecordTypeA, "10.0.0.1", nil, 1800),
		detailed("www", RecordTypeA, "10.0.0.2", nil, 1800),
	})
	// Another writer adds a record after every setHosts, so verify never matches.
	m.afterSet = func(m *dnsMock, _ int) {
		m.zone = append(m.zone, detailed("gremlin", RecordTypeA, "10.0.0.99", nil, 1800))
	}
	client, server := mockedClient(m)
	defer server.Close()

	_, err := client.DomainsDNS.DeleteRecordsWithContext(context.Background(), "domain.net",
		RecordSelector{HostName: String("www"), RecordType: String(RecordTypeA)})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrConcurrentModification)
	assert.Equal(t, 1, m.setCount) // no retry by default
}

func TestRetryOnConflictSucceedsSecondAttempt(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeNone, []DomainsDNSHostRecordDetailed{
		detailed("@", RecordTypeA, "10.0.0.1", nil, 1800),
		detailed("www", RecordTypeA, "10.0.0.2", nil, 1800),
	})
	// Inject a conflicting write only after the first setHosts; the second
	// attempt's verify then matches.
	m.afterSet = func(m *dnsMock, setIndex int) {
		if setIndex == 1 {
			m.zone = append(m.zone, detailed("gremlin", RecordTypeA, "10.0.0.99", nil, 1800))
		}
	}
	client, server := mockedClient(m)
	defer server.Close()

	res, err := client.DomainsDNS.DeleteRecordsWithContext(context.Background(), "domain.net",
		RecordSelector{HostName: String("www"), RecordType: String(RecordTypeA)},
		WithRetryOnConflict(2))
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 2, m.setCount) // succeeded on the second attempt
}

func TestDeleteRecordsEmptySelectorRejected(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeNone, []DomainsDNSHostRecordDetailed{
		detailed("@", RecordTypeA, "10.0.0.1", nil, 1800),
	})
	client, server := mockedClient(m)
	defer server.Close()

	_, err := client.DomainsDNS.DeleteRecordsWithContext(context.Background(), "domain.net", RecordSelector{})
	assert.Error(t, err)
	var argErr *InvalidArgumentsError
	assert.ErrorAs(t, err, &argErr)
	// No HTTP call was made.
	assert.Equal(t, 0, m.getCount)
	assert.Equal(t, 0, m.setCount)
}

func TestDeleteRecordsMatchAllWipe(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeNone, []DomainsDNSHostRecordDetailed{
		detailed("@", RecordTypeA, "10.0.0.1", nil, 1800),
		detailed("www", RecordTypeA, "10.0.0.2", nil, 1800),
	})
	client, server := mockedClient(m)
	defer server.Close()

	res, err := client.DomainsDNS.DeleteRecordsWithContext(context.Background(), "domain.net", RecordSelector{MatchAll: true})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 2, res.Removed)
	assert.Empty(t, res.Records)

	m.mu.Lock()
	remaining := len(m.zone)
	m.mu.Unlock()
	assert.Equal(t, 0, remaining)
}

func TestSelectorMatching(t *testing.T) {
	t.Parallel()
	rec := DomainsDNSHostRecord{
		HostName: String("www"), RecordType: String(RecordTypeA),
		Address: String("10.0.0.1"), TTL: Int(1800),
	}
	mxRec := DomainsDNSHostRecord{
		HostName: String("mail"), RecordType: String(RecordTypeMX),
		Address: String("mailhost.example.com"), MXPref: UInt8(10), TTL: Int(1800),
	}
	cases := []struct {
		name     string
		selector RecordSelector
		record   DomainsDNSHostRecord
		want     bool
	}{
		{"hostname match", RecordSelector{HostName: String("www")}, rec, true},
		{"hostname case-insensitive", RecordSelector{HostName: String("WWW")}, rec, true},
		{"hostname mismatch", RecordSelector{HostName: String("blog")}, rec, false},
		{"type match case-insensitive", RecordSelector{RecordType: String("a")}, rec, true},
		{"type mismatch", RecordSelector{RecordType: String(RecordTypeCNAME)}, rec, false},
		{"address match", RecordSelector{Address: String("10.0.0.1")}, rec, true},
		{"address trailing dot ignored", RecordSelector{Address: String("mailhost.example.com.")}, mxRec, true},
		{"combined match", RecordSelector{HostName: String("www"), RecordType: String(RecordTypeA), Address: String("10.0.0.1")}, rec, true},
		{"combined one field mismatch", RecordSelector{HostName: String("www"), RecordType: String(RecordTypeA), Address: String("10.0.0.2")}, rec, false},
		{"mxpref match", RecordSelector{MXPref: UInt8(10)}, mxRec, true},
		{"mxpref mismatch", RecordSelector{MXPref: UInt8(20)}, mxRec, false},
		{"mxpref against nil record", RecordSelector{MXPref: UInt8(10)}, rec, false},
		{"matchall", RecordSelector{MatchAll: true}, rec, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.selector.matches(tc.record))
		})
	}
}

func TestPlan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		ops        []RecordOp
		wantAdd    int
		wantRemove int
		wantKeep   int
	}{
		{
			name:    "add",
			ops:     []RecordOp{AddOp(DomainsDNSHostRecord{HostName: String("new"), RecordType: String(RecordTypeA), Address: String("10.0.0.9"), TTL: Int(1800)})},
			wantAdd: 1, wantRemove: 0, wantKeep: 12,
		},
		{
			name:    "delete",
			ops:     []RecordOp{DeleteOp(RecordSelector{RecordType: String(RecordTypeTXT)})},
			wantAdd: 0, wantRemove: 1, wantKeep: 11,
		},
		{
			name: "upsert",
			ops: []RecordOp{UpsertOp(
				RecordSelector{HostName: String("blog"), RecordType: String(RecordTypeCNAME)},
				[]DomainsDNSHostRecord{{HostName: String("blog"), RecordType: String(RecordTypeCNAME), Address: String("newhost.example.com"), TTL: Int(1800)}},
			)},
			wantAdd: 1, wantRemove: 1, wantKeep: 11,
		},
		{
			name:    "no-op",
			ops:     []RecordOp{DeleteOp(RecordSelector{HostName: String("doesnotexist")})},
			wantAdd: 0, wantRemove: 0, wantKeep: 12,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newDNSMock(EmailTypeMX, fixtureEveryType())
			client, server := mockedClient(m)
			defer server.Close()

			diff, err := client.DomainsDNS.PlanWithContext(context.Background(), "domain.net", tc.ops...)
			assert.NoError(t, err)
			assert.Len(t, diff.Add, tc.wantAdd)
			assert.Len(t, diff.Remove, tc.wantRemove)
			assert.Len(t, diff.Keep, tc.wantKeep)
			// Plan performs a single read and zero writes.
			assert.Equal(t, 1, m.getCount)
			assert.Equal(t, 0, m.setCount)
			assert.Contains(t, diff.String(), "RecordDiff:")
		})
	}
}

func TestPlanEmptySelectorRejected(t *testing.T) {
	t.Parallel()
	m := newDNSMock(EmailTypeMX, fixtureAllTypes())
	client, server := mockedClient(m)
	defer server.Close()

	_, err := client.DomainsDNS.PlanWithContext(context.Background(), "domain.net", DeleteOp(RecordSelector{}))
	assert.Error(t, err)
	var argErr *InvalidArgumentsError
	assert.ErrorAs(t, err, &argErr)
	assert.Equal(t, 0, m.getCount)
	assert.Equal(t, 0, m.setCount)
}

func TestNormalizeRecord(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      DomainsDNSHostRecord
		host    string
		rtype   string
		address string
		ttl     int
	}{
		{"nil ttl defaults to 1799", DomainsDNSHostRecord{HostName: String("@"), RecordType: String(RecordTypeA), Address: String("1.2.3.4")}, "@", "A", "1.2.3.4", 1799},
		{"zero ttl defaults to 1799", DomainsDNSHostRecord{HostName: String("@"), RecordType: String(RecordTypeA), Address: String("1.2.3.4"), TTL: Int(0)}, "@", "A", "1.2.3.4", 1799},
		{"explicit ttl kept", DomainsDNSHostRecord{HostName: String("@"), RecordType: String(RecordTypeA), Address: String("1.2.3.4"), TTL: Int(3600)}, "@", "A", "1.2.3.4", 3600},
		{"host lower-cased and type upper-cased", DomainsDNSHostRecord{HostName: String("WWW"), RecordType: String("a"), Address: String("1.2.3.4")}, "www", "A", "1.2.3.4", 1799},
		{"empty host becomes apex", DomainsDNSHostRecord{HostName: String(""), RecordType: String(RecordTypeA), Address: String("1.2.3.4")}, "@", "A", "1.2.3.4", 1799},
		{"trailing dot trimmed from address", DomainsDNSHostRecord{HostName: String("blog"), RecordType: String(RecordTypeCNAME), Address: String("target.example.com.")}, "blog", "CNAME", "target.example.com", 1799},
		{"txt address case preserved", DomainsDNSHostRecord{HostName: String("@"), RecordType: String(RecordTypeTXT), Address: String("V=spf1")}, "@", "TXT", "V=spf1", 1799},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := NormalizeRecord(tc.in)
			assert.Equal(t, tc.host, *out.HostName)
			assert.Equal(t, tc.rtype, *out.RecordType)
			assert.Equal(t, tc.address, *out.Address)
			assert.Equal(t, tc.ttl, *out.TTL)
		})
	}
}

func TestNormalizeRecordDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	host := "WWW"
	addr := "target.com."
	in := DomainsDNSHostRecord{HostName: &host, RecordType: String("a"), Address: &addr}
	_ = NormalizeRecord(in)
	assert.Equal(t, "WWW", host)
	assert.Equal(t, "target.com.", addr)
}

func TestRecordsEqual(t *testing.T) {
	t.Parallel()
	a := DomainsDNSHostRecord{HostName: String("WWW"), RecordType: String("a"), Address: String("target.com."), TTL: Int(0)}
	b := DomainsDNSHostRecord{HostName: String("www"), RecordType: String(RecordTypeA), Address: String("target.com"), TTL: Int(1799)}
	assert.True(t, RecordsEqual(a, b))
	assert.True(t, RecordsEqual(b, a)) // symmetric

	c := DomainsDNSHostRecord{HostName: String("www"), RecordType: String(RecordTypeA), Address: String("other.com")}
	assert.False(t, RecordsEqual(a, c))
	assert.False(t, RecordsEqual(c, a))

	// TTL is part of identity.
	d := DomainsDNSHostRecord{HostName: String("www"), RecordType: String(RecordTypeA), Address: String("target.com"), TTL: Int(60)}
	assert.False(t, RecordsEqual(b, d))
}
