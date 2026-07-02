package namecheaptest

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// fixturesFS embeds the sandbox-captured success fixtures. Every implemented
// Namecheap command has exactly one fixture here, named by the command with the
// "namecheap." prefix stripped and the remaining dots replaced by underscores
// (e.g. "namecheap.domains.getInfo" -> "domains_getInfo.xml"). The completeness
// test in this package enforces that the corpus stays in sync with the command
// surface of the core package.
//
//go:embed fixtures/*.xml
var fixturesFS embed.FS

// fixtureName maps a fixture short name to its path inside the embedded FS.
func fixtureName(name string) string {
	name = strings.TrimSuffix(name, ".xml")
	return "fixtures/" + name + ".xml"
}

// FixtureOK returns the body of the embedded success fixture identified by name.
//
// name is the fixture short name: the command with the leading "namecheap."
// stripped and dots replaced by underscores (e.g. "domains_getInfo" for
// namecheap.domains.getInfo), with or without a trailing ".xml". It is designed
// to be handed straight to Server.Stub:
//
//	srv.Stub("namecheap.domains.getInfo", namecheaptest.FixtureOK("domains_getInfo"))
//
// FixtureOK panics if no such fixture exists, so a typo fails the test loudly
// and immediately rather than silently stubbing an empty body.
func FixtureOK(name string) string {
	data, err := fixturesFS.ReadFile(fixtureName(name))
	if err != nil {
		return panicUnknownFixture(name)
	}
	return string(data)
}

// panicUnknownFixture reports an unknown fixture name together with the list of
// available names, to make the failure self-explanatory.
func panicUnknownFixture(name string) string {
	panic(fmt.Sprintf("namecheaptest: unknown fixture %q; available: %s",
		name, strings.Join(FixtureNames(), ", ")))
}

// FixtureNames returns the sorted short names of every embedded fixture (without
// the "fixtures/" prefix or ".xml" suffix). It is useful for diagnostics and for
// tests that iterate the corpus.
func FixtureNames() []string {
	entries, err := fs.ReadDir(fixturesFS, "fixtures")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".xml"))
	}
	sort.Strings(names)
	return names
}

// commandToFixture converts a full Namecheap command string into its fixture
// short name using the corpus naming scheme (strip "namecheap.", dots to
// underscores). It is the single source of truth shared by the helpers and the
// completeness test.
func commandToFixture(command string) string {
	return strings.ReplaceAll(strings.TrimPrefix(command, "namecheap."), ".", "_")
}
