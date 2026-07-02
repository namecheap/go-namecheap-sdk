package namecheaptest

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
)

// commandLiteral matches the `"Command": "namecheap.xxx"` literals that every
// command implementation carries, which is the authoritative source of the
// command surface. Deriving the corpus scope from it keeps the completeness
// check in sync with the code automatically instead of hard-coding a list.
var commandLiteral = regexp.MustCompile(`"Command":\s*"(namecheap\.[a-zA-Z.]+)"`)

const coreSrcDir = "../namecheap"

// implementedCommands scans the core package's non-test sources for the Command
// literals and returns the sorted, de-duplicated set.
func implementedCommands(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(coreSrcDir)
	if err != nil {
		t.Fatalf("reading core package dir %q: %v", coreSrcDir, err)
	}
	set := make(map[string]struct{})
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(coreSrcDir, name))
		if err != nil {
			t.Fatalf("reading %q: %v", name, err)
		}
		for _, m := range commandLiteral.FindAllStringSubmatch(string(data), -1) {
			set[m[1]] = struct{}{}
		}
	}
	cmds := make([]string, 0, len(set))
	for c := range set {
		cmds = append(cmds, c)
	}
	sort.Strings(cmds)
	return cmds
}

// wellFormed reports an error if data is not well-formed XML.
func wellFormed(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// TestFixtureCorpusCompleteness fails if any implemented command lacks a
// loadable, well-formed success fixture. It is what forces future command
// coverage to ship a fixture through this package.
func TestFixtureCorpusCompleteness(t *testing.T) {
	t.Parallel()
	cmds := implementedCommands(t)
	if len(cmds) == 0 {
		t.Fatal("no commands discovered in core package — the scan is broken")
	}
	for _, cmd := range cmds {
		short := commandToFixture(cmd)
		body, err := fixturesFS.ReadFile(fixtureName(short))
		if err != nil {
			t.Errorf("missing fixture for command %q (expected fixtures/%s.xml)", cmd, short)
			continue
		}
		if err := wellFormed(body); err != nil {
			t.Errorf("fixture %s.xml for command %q is not well-formed XML: %v", short, cmd, err)
		}
	}
	t.Logf("verified success fixtures for %d implemented commands", len(cmds))
}

// TestEveryFixtureIsWellFormedOKEnvelope asserts every embedded fixture is
// well-formed XML carrying Status="OK".
func TestEveryFixtureIsWellFormedOKEnvelope(t *testing.T) {
	t.Parallel()
	names := FixtureNames()
	if len(names) == 0 {
		t.Fatal("no fixtures embedded")
	}
	for _, name := range names {
		body := []byte(FixtureOK(name))
		if err := wellFormed(body); err != nil {
			t.Errorf("fixture %s: not well-formed XML: %v", name, err)
			continue
		}
		var env struct {
			Status string `xml:"Status,attr"`
		}
		if err := xml.Unmarshal(body, &env); err != nil {
			t.Errorf("fixture %s: cannot decode envelope: %v", name, err)
			continue
		}
		if env.Status != "OK" {
			t.Errorf("fixture %s: Status=%q, want \"OK\"", name, env.Status)
		}
	}
}

// TestNoOrphanFixtures keeps the corpus bidirectional: every fixture must map to
// an implemented command, so stale fixtures do not linger after a command is
// removed.
func TestNoOrphanFixtures(t *testing.T) {
	t.Parallel()
	valid := make(map[string]struct{})
	for _, cmd := range implementedCommands(t) {
		valid[commandToFixture(cmd)] = struct{}{}
	}
	for _, name := range FixtureNames() {
		if _, ok := valid[name]; !ok {
			t.Errorf("orphan fixture %s.xml does not correspond to any implemented command", name)
		}
	}
}

// TestRepresentativeFixturesDecodeIntoStructs proves a handful of fixtures
// decode into the real SDK response structs with their key fields populated,
// guarding against a fixture that is well-formed but structurally wrong.
func TestRepresentativeFixturesDecodeIntoStructs(t *testing.T) {
	t.Parallel()

	t.Run("domains_getInfo", func(t *testing.T) {
		t.Parallel()
		var resp namecheap.DomainsGetInfoResponse
		if err := xml.Unmarshal([]byte(FixtureOK("domains_getInfo")), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.CommandResponse == nil || resp.CommandResponse.DomainDNSGetListResult == nil {
			t.Fatal("CommandResponse/DomainGetInfoResult not populated")
		}
		if got := resp.CommandResponse.DomainDNSGetListResult.DomainName; got == nil || *got == "" {
			t.Errorf("DomainName not populated: %v", got)
		}
	})

	t.Run("domains_getList", func(t *testing.T) {
		t.Parallel()
		var resp namecheap.DomainsGetListResponse
		if err := xml.Unmarshal([]byte(FixtureOK("domains_getList")), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.CommandResponse == nil || resp.CommandResponse.Domains == nil || len(*resp.CommandResponse.Domains) == 0 {
			t.Fatal("expected at least one Domain in getList fixture")
		}
	})

	t.Run("ssl_getInfo", func(t *testing.T) {
		t.Parallel()
		var resp namecheap.SSLGetInfoResponse
		if err := xml.Unmarshal([]byte(FixtureOK("ssl_getInfo")), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.CommandResponse == nil || resp.CommandResponse.SSLGetInfoResult == nil {
			t.Fatal("SSLGetInfoResult not populated")
		}
		if got := resp.CommandResponse.SSLGetInfoResult.CertificateID; got == nil || *got == 0 {
			t.Errorf("CertificateID not populated: %v", got)
		}
	})
}
