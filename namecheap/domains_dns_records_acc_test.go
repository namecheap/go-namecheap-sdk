//go:build acceptance

package namecheap_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
)

// TestAcc_EmailType pins, against the live API, the setHosts behaviour the
// record helpers' EmailType derivation is built on: omission preserves the zone
// mode, MX rows need MX, no MX/MXE count rules, an unknown EmailType answers
// IsSuccess="false", and preset zones (GMAIL, OX, FWD) are writable without an
// EmailType.
//
// It switches the mail mode of NAMECHEAP_TEST_DOMAIN, which must be a
// disposable domain whose mail nobody relies on, on the sandbox or production
// alike. The zone's records and mode are captured first and restored on exit;
// rows a mail preset created (tagged with AssociatedAppTitle) come back as plain
// rows. The subtests share the zone and run in order.
//
// ErrUnverifiedWrite (#193) is not covered here: a failing verify read cannot be
// provoked on demand against the live API. It is covered by TestUnverifiedWrite.
func TestAcc_EmailType(t *testing.T) {
	domain := os.Getenv(envDomain)
	client := liveClient(t)
	if domain == "" {
		t.Skipf("%s not set; skipping EmailType acceptance", envDomain)
	}
	dns := client.DomainsDNS

	original := readZone(t, client, domain)
	t.Cleanup(func() {
		records := original.records
		if _, err := dns.SetHostsWithContext(context.Background(), &namecheap.DomainsDNSSetHostsArgs{
			Domain:    namecheap.String(domain),
			Records:   &records,
			EmailType: namecheap.String(original.emailType),
		}); err != nil {
			t.Errorf("restore %s to EmailType=%s with %d records failed (zone may be modified): %v",
				domain, original.emailType, len(records), err)
		}
	})

	apex := namecheap.DomainsDNSHostRecord{
		HostName: namecheap.String("@"), RecordType: namecheap.String(namecheap.RecordTypeA),
		Address: namecheap.String("199.34.228.77"), TTL: namecheap.Int(1799),
	}
	mx := namecheap.DomainsDNSHostRecord{
		HostName: namecheap.String("@"), RecordType: namecheap.String(namecheap.RecordTypeMX),
		Address: namecheap.String("mx-acc.example.com."), MXPref: namecheap.UInt8(10), TTL: namecheap.Int(1799),
	}
	mxe := namecheap.DomainsDNSHostRecord{
		HostName: namecheap.String("@"), RecordType: namecheap.String(namecheap.RecordTypeMXE),
		Address: namecheap.String("199.34.228.78"), TTL: namecheap.Int(1799),
	}
	txt := namecheap.DomainsDNSHostRecord{
		HostName: namecheap.String("_acc-emailtype"), RecordType: namecheap.String(namecheap.RecordTypeTXT),
		Address: namecheap.String("acceptance"), TTL: namecheap.Int(300),
	}
	mxSel := namecheap.RecordSelector{RecordType: namecheap.String(namecheap.RecordTypeMX)}
	mxeSel := namecheap.RecordSelector{RecordType: namecheap.String(namecheap.RecordTypeMXE)}
	txtSel := namecheap.RecordSelector{HostName: txt.HostName, RecordType: txt.RecordType}

	// reset puts the zone into a known state: the apex A record under emailType.
	reset := func(t *testing.T, emailType string) {
		t.Helper()
		records := []namecheap.DomainsDNSHostRecord{apex}
		if _, err := dns.SetHostsWithContext(ctx(t), &namecheap.DomainsDNSSetHostsArgs{
			Domain: namecheap.String(domain), Records: &records, EmailType: namecheap.String(emailType),
		}); err != nil {
			t.Fatalf("reset to EmailType=%s: %v", emailType, err)
		}
	}

	t.Run("unknown EmailType answers IsSuccess=false and changes nothing", func(t *testing.T) {
		reset(t, namecheap.EmailTypeNone)
		// SetHosts rejects an unknown value client-side, so send it raw: this pins
		// the response shape SetHostsWithContext's IsSuccess check relies on.
		resp := rawSetHosts(t, client, domain, map[string]string{
			"EmailType": "BOGUS", "HostName1": "@", "RecordType1": "A", "Address1": "199.34.228.99",
		})
		result := resp.CommandResponse.DomainDNSSetHostsResult
		if result == nil || result.IsSuccess == nil || *result.IsSuccess {
			t.Fatalf("setHosts EmailType=BOGUS result = %v, want IsSuccess=false", result)
		}
		z := readZone(t, client, domain)
		wantZone(t, z, namecheap.EmailTypeNone, []namecheap.DomainsDNSHostRecord{apex})
	})

	t.Run("first MX on a NONE zone switches the zone to MX", func(t *testing.T) {
		if _, err := dns.AddRecordsWithContext(ctx(t), domain, []namecheap.DomainsDNSHostRecord{mx}); err != nil {
			t.Fatalf("AddRecords(MX) on a NONE zone: %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMX, []namecheap.DomainsDNSHostRecord{apex, mx})
	})

	t.Run("plain change on an MX zone keeps the MX row", func(t *testing.T) {
		if _, err := dns.AddRecordsWithContext(ctx(t), domain, []namecheap.DomainsDNSHostRecord{txt}); err != nil {
			t.Fatalf("AddRecords(TXT) on an MX zone: %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMX, []namecheap.DomainsDNSHostRecord{apex, mx, txt})
	})

	t.Run("MX row with EmailType omitted is kept on an MX zone", func(t *testing.T) {
		// SetHosts requires EmailType=MX with MX records because it cannot see the
		// zone's mode, so send it raw: this pins that omission preserves MX.
		rawSetHosts(t, client, domain, map[string]string{
			"HostName1": "@", "RecordType1": "A", "Address1": *apex.Address, "TTL1": "1799",
			"HostName2": "@", "RecordType2": "MX", "Address2": *mx.Address, "MXPref2": "10", "TTL2": "1799",
		})
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMX, []namecheap.DomainsDNSHostRecord{apex, mx})
	})

	t.Run("MXE row is kept when EmailType is MX", func(t *testing.T) {
		records := []namecheap.DomainsDNSHostRecord{apex, mx, mxe}
		if _, err := dns.SetHostsWithContext(ctx(t), &namecheap.DomainsDNSSetHostsArgs{
			Domain: namecheap.String(domain), Records: &records, EmailType: namecheap.String(namecheap.EmailTypeMX),
		}); err != nil {
			t.Fatalf("SetHosts(MX+MXE, EmailType=MX): %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMX, records)
	})

	t.Run("deleting the MX rows leaves the zone on MX, MXE rows or not", func(t *testing.T) {
		if _, err := dns.DeleteRecordsWithContext(ctx(t), domain, mxSel); err != nil {
			t.Fatalf("DeleteRecords(MX) with an MXE row left: %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMX, []namecheap.DomainsDNSHostRecord{apex, mxe})

		if _, err := dns.DeleteRecordsWithContext(ctx(t), domain, mxeSel); err != nil {
			t.Fatalf("DeleteRecords(MXE): %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMX, []namecheap.DomainsDNSHostRecord{apex})
	})

	t.Run("WithEmailType overrides the derivation", func(t *testing.T) {
		if _, err := dns.DeleteRecordsWithContext(ctx(t), domain, txtSel, namecheap.WithEmailType(namecheap.EmailTypeNone)); err != nil {
			t.Fatalf("DeleteRecords(TXT, WithEmailType(NONE)): %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeNone, []namecheap.DomainsDNSHostRecord{apex})
	})

	t.Run("first MXE on a NONE zone switches the zone to MXE", func(t *testing.T) {
		if _, err := dns.AddRecordsWithContext(ctx(t), domain, []namecheap.DomainsDNSHostRecord{mxe}); err != nil {
			t.Fatalf("AddRecords(MXE) on a NONE zone: %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMXE, []namecheap.DomainsDNSHostRecord{apex, mxe})

		if _, err := dns.DeleteRecordsWithContext(ctx(t), domain, mxeSel); err != nil {
			t.Fatalf("DeleteRecords(last MXE): %v", err)
		}
		wantZone(t, readZone(t, client, domain), namecheap.EmailTypeMXE, []namecheap.DomainsDNSHostRecord{apex})
	})

	// #162 and the #355 reporter: a per-record write on a preset zone used to be
	// rejected client-side. It must now go through without touching the mode.
	for _, preset := range []string{namecheap.EmailTypeGmail, namecheap.EmailTypePrivate, namecheap.EmailTypeForward} {
		t.Run("record write on a "+preset+" zone keeps the preset", func(t *testing.T) {
			reset(t, preset)
			if _, err := dns.AddRecordsWithContext(ctx(t), domain, []namecheap.DomainsDNSHostRecord{txt}); err != nil {
				t.Fatalf("AddRecords(TXT) on a %s zone: %v", preset, err)
			}
			if _, err := dns.DeleteRecordsWithContext(ctx(t), domain, txtSel); err != nil {
				t.Fatalf("DeleteRecords(TXT) on a %s zone: %v", preset, err)
			}
			z := readZone(t, client, domain)
			if z.emailType != preset {
				t.Errorf("EmailType after add+delete = %s, want %s", z.emailType, preset)
			}
		})
	}

	t.Run("preset tag on the FWD SPF row is read and survives a write", func(t *testing.T) {
		reset(t, namecheap.EmailTypeNone)
		// The tagged SPF row is created only by the undocumented setEmailType
		// command; setHosts never creates preset rows.
		var void struct{}
		if _, err := client.DoXMLWithContext(ctx(t), map[string]string{
			"Command": "namecheap.domains.dns.setEmailType", "DomainName": domain, "EmailType": namecheap.EmailTypeForward,
		}, &void); err != nil {
			t.Skipf("setEmailType unavailable, cannot create a tagged row: %v", err)
		}
		before := readZone(t, client, domain)
		if before.tagged() == 0 {
			t.Skip("setEmailType FWD created no tagged row on this gateway")
		}

		res, err := dns.AddRecordsWithContext(ctx(t), domain, []namecheap.DomainsDNSHostRecord{txt})
		if err != nil {
			t.Fatalf("AddRecords(TXT) on a FWD zone with a tagged row: %v", err)
		}
		if got := countTagged(res.Records); got != before.tagged() {
			t.Errorf("RecordChangeResult.Records carries %d tagged rows, want %d", got, before.tagged())
		}
		after := readZone(t, client, domain)
		if after.emailType != namecheap.EmailTypeForward {
			t.Errorf("EmailType = %s, want FWD", after.emailType)
		}
		if after.tagged() != before.tagged() {
			t.Errorf("tagged rows after the write = %d, want %d (an echoed row must keep its tag)", after.tagged(), before.tagged())
		}
	})
}

// rawSetHosts sends setHosts for domain with params, bypassing the SDK's
// client-side validation, and fails the test on an API error.
func rawSetHosts(t *testing.T, client *namecheap.Client, domain string, params map[string]string) namecheap.DomainsDNSSetHostsResponse {
	t.Helper()
	parsed, err := namecheap.ParseDomain(domain)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"Command": "namecheap.domains.dns.setHosts", "SLD": parsed.SLD, "TLD": parsed.TLD}
	for k, v := range params {
		body[k] = v
	}
	var resp namecheap.DomainsDNSSetHostsResponse
	if _, err := client.DoXMLWithContext(ctx(t), body, &resp); err != nil {
		t.Fatalf("raw setHosts %v: %v", params, err)
	}
	return resp
}

type zoneState struct {
	emailType string
	records   []namecheap.DomainsDNSHostRecord
}

func (z zoneState) tagged() int { return countTagged(z.records) }

func countTagged(records []namecheap.DomainsDNSHostRecord) int {
	n := 0
	for _, r := range records {
		if r.AssociatedAppTitle != nil && *r.AssociatedAppTitle != "" {
			n++
		}
	}
	return n
}

func readZone(t *testing.T, client *namecheap.Client, domain string) zoneState {
	t.Helper()
	resp, err := client.DomainsDNS.GetHostsWithContext(ctx(t), domain)
	if err != nil {
		t.Fatalf("getHosts %s: %v", domain, err)
	}
	z := zoneState{emailType: namecheap.EmailTypeNone}
	if r := resp.DomainDNSGetHostsResult; r != nil {
		if r.EmailType != nil && *r.EmailType != "" {
			z.emailType = *r.EmailType
		}
		if r.Hosts != nil {
			for _, h := range *r.Hosts {
				z.records = append(z.records, namecheap.RecordFromDetailed(h))
			}
		}
	}
	return z
}

// wantZone compares a zone read against the expected mode and record multiset.
func wantZone(t *testing.T, got zoneState, emailType string, records []namecheap.DomainsDNSHostRecord) {
	t.Helper()
	if got.emailType != emailType {
		t.Errorf("zone EmailType = %s, want %s", got.emailType, emailType)
	}
	remaining := append([]namecheap.DomainsDNSHostRecord(nil), got.records...)
	for _, w := range records {
		i := indexEqual(remaining, w)
		if i < 0 {
			t.Errorf("zone is missing %s", describe(w))
			continue
		}
		remaining = append(remaining[:i], remaining[i+1:]...)
	}
	for _, r := range remaining {
		t.Errorf("zone has unexpected %s", describe(r))
	}
}

func indexEqual(records []namecheap.DomainsDNSHostRecord, want namecheap.DomainsDNSHostRecord) int {
	for i, r := range records {
		if namecheap.RecordsEqual(r, want) {
			return i
		}
	}
	return -1
}

func describe(r namecheap.DomainsDNSHostRecord) string {
	n := namecheap.NormalizeRecord(r)
	s := *n.HostName + " " + *n.RecordType + " " + *n.Address + " ttl=" + strconv.Itoa(*n.TTL)
	if n.MXPref != nil {
		s += " mxpref=" + strconv.Itoa(int(*n.MXPref))
	}
	return s
}
