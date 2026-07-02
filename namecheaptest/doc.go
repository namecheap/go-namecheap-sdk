// Package namecheaptest provides a mock Namecheap API server and a shipped
// corpus of success fixtures for testing code that uses the go-namecheap-sdk.
//
// It exists so that every consumer of the SDK — the Terraform provider, an
// MCP server, internal tooling — stops rebuilding the same httptest scaffolding
// and hand-captured XML by hand. Point the SDK at a Server, stub the commands
// your code exercises, run it, and assert on what was sent.
//
// # Worked example
//
// The mock routes on the Command form field, so one Server serves every command
// under test:
//
//	func TestCreatesThenReadsBack(t *testing.T) {
//		srv := namecheaptest.NewServer(t) // httptest-backed, auto-closed via t.Cleanup
//
//		// Stub responses by command. Use a shipped fixture, an inline body, or a
//		// synthesized API error.
//		srv.StubFixture("namecheap.domains.getInfo", "domains_getInfo")
//		srv.Stub("namecheap.domains.getContacts", namecheaptest.FixtureOK("domains_getContacts"))
//		srv.StubError("namecheap.domains.create", 2019166) // errors.As -> *namecheap.APIError{Number: 2019166}
//
//		client := srv.Client() // pre-wired *namecheap.Client pointed at the mock
//
//		// Exercise the code under test.
//		info, err := client.Domains.GetInfoWithContext(context.Background(), "example.com")
//		if err != nil {
//			t.Fatal(err)
//		}
//		_ = info
//
//		// Assert on the request the SDK actually sent.
//		srv.AssertCalled(t, "namecheap.domains.getInfo", map[string]string{
//			"DomainName": "example.com",
//		})
//	}
//
// # Sequences
//
// StubSequence returns a different body per call (the last entry repeats), which
// models read-modify-write and polling flows:
//
//	srv.StubSequence("namecheap.domains.transfer.getStatus",
//		namecheaptest.FixtureOK("domains_transfer_getStatus"), // first poll
//		pendingBody, doneBody)                                 // then these
//
// # Fixtures
//
// The corpus under fixtures/ carries one success fixture per implemented
// command, named by stripping the "namecheap." prefix and replacing dots with
// underscores (namecheap.domains.getInfo -> domains_getInfo.xml). FixtureOK
// returns a fixture body by that short name and panics on a typo. A completeness
// test fails if any command lacks a fixture, so new command coverage must ship
// its fixture.
//
// # Compatibility
//
// namecheaptest is part of the SDK's public API surface. Breaking changes to it
// follow the same semantic-versioning rules as the core namecheap package: they
// only happen in a major release. Consumers may depend on the exported API and
// on the fixture naming scheme with the same stability guarantees.
package namecheaptest
