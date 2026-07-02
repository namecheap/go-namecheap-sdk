package namecheaptest

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"

	namecheap "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
)

// Test credentials wired into every Client returned by Server.Client. They are
// deliberately obvious placeholders: the mock never validates them, and they must
// never resemble real credentials.
const (
	testUserName = "testuser"
	testAPIUser  = "testuser"
	testAPIKey   = "testkey"
	testClientIP = "127.0.0.1"
)

// Server is an httptest-backed mock of the Namecheap API. It routes requests on
// the "Command" form field, so a single Server can serve every command a test
// exercises. Register responses with Stub, StubFixture, StubError or
// StubSequence, then obtain a pre-wired client with Client. After exercising the
// code under test, inspect what was sent with Calls or AssertCalled.
//
// A Server is safe for concurrent use: the route table and call log are guarded
// by a mutex, so tests may drive it from parallel goroutines.
type Server struct {
	http *httptest.Server

	mu     sync.Mutex
	routes map[string]*stub
	calls  map[string][]map[string]string
	client *namecheap.Client

	closeOnce sync.Once
}

// stub is a registered response (or sequence of responses) for one command.
type stub struct {
	bodies []string
	idx    int
}

// NewServer starts a mock API server and registers its shutdown with
// t.Cleanup, so callers never have to close it explicitly. The returned Server
// has no routes; register them before exercising the code under test.
func NewServer(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		routes: make(map[string]*stub),
		calls:  make(map[string][]map[string]string),
	}
	s.http = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

// URL returns the base URL of the running mock server. Client already points at
// it; URL is exposed for advanced wiring (e.g. a hand-built *namecheap.Client).
func (s *Server) URL() string {
	return s.http.URL
}

// Close shuts the mock server down. It is registered with t.Cleanup by
// NewServer and is safe to call more than once.
func (s *Server) Close() {
	s.closeOnce.Do(s.http.Close)
}

// Client returns a *namecheap.Client pre-wired to this server with test
// credentials. Client-side rate limiting is disabled and the retry policy is
// reduced to a single attempt, so tests are fast and each SDK call maps to
// exactly one recorded request (making Calls and AssertCalled deterministic).
// The same client is returned on repeated calls. It needs no cleanup of its own;
// the server's t.Cleanup shutdown covers the whole lifecycle.
func (s *Server) Client() *namecheap.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		s.client = namecheap.NewClient(&namecheap.ClientOptions{
			UserName:  testUserName,
			ApiUser:   testAPIUser,
			ApiKey:    testAPIKey,
			ClientIp:  testClientIP,
			RateLimit: &namecheap.RateLimitOptions{Disabled: true},
			Retry:     &namecheap.RetryOptions{MaxAttempts: 1},
		})
		s.client.BaseURL = s.http.URL
	}
	return s.client
}

// Stub registers body as the HTTP 200 response for command. Registering a
// command again replaces any previous stub or sequence for it.
func (s *Server) Stub(command, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[command] = &stub{bodies: []string{body}}
}

// StubFixture registers the embedded success fixture fixtureName as the response
// for command. It is shorthand for Stub(command, FixtureOK(fixtureName)) and
// panics if the fixture does not exist.
func (s *Server) StubFixture(command, fixtureName string) {
	s.Stub(command, FixtureOK(fixtureName))
}

// StubError registers a synthesized Namecheap ERROR envelope for command, so the
// SDK surfaces it as an *namecheap.APIError whose Number is number (match it with
// errors.As). An optional message overrides the default; only the first value is
// used. No per-command error fixture is needed.
func (s *Server) StubError(command string, number int, message ...string) {
	msg := "stubbed error"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	s.Stub(command, errorEnvelope(number, msg))
}

// StubSequence registers an ordered list of responses for command: the first
// call returns bodies[0], the second bodies[1], and so on; once the list is
// exhausted the last entry repeats. It models read-modify-write and polling
// flows where successive calls to the same command must return different bodies.
// Calling it with no bodies registers a single empty response.
func (s *Server) StubSequence(command string, bodies ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(bodies) == 0 {
		bodies = []string{""}
	}
	s.routes[command] = &stub{bodies: append([]string(nil), bodies...)}
}

// Calls returns a copy of the captured form-parameter maps for every request
// made to command, in call order. It returns an empty slice when the command was
// never called. The credential fields the SDK injects are included verbatim.
func (s *Server) Calls(command string) []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	recorded := s.calls[command]
	out := make([]map[string]string, 0, len(recorded))
	for _, c := range recorded {
		out = append(out, cloneParams(c))
	}
	return out
}

// AssertCalled fails the test unless command was called at least once with form
// parameters that include (at least) every key/value pair in wantParams. Extra
// parameters on the request are ignored, so callers assert only the fields they
// care about. A nil or empty wantParams asserts merely that the command was
// called.
func (s *Server) AssertCalled(t testing.TB, command string, wantParams map[string]string) {
	t.Helper()
	calls := s.Calls(command)
	if len(calls) == 0 {
		t.Errorf("namecheaptest: expected command %q to be called, but it was not; called commands: %v",
			command, s.calledCommands())
		return
	}
	if len(wantParams) == 0 {
		return
	}
	for _, c := range calls {
		if matchesSubset(c, wantParams) {
			return
		}
	}
	t.Errorf("namecheaptest: command %q was called %d time(s) but none matched params %v; captured calls: %v",
		command, len(calls), wantParams, calls)
}

// handle is the HTTP handler that routes on the Command form field, captures the
// request parameters and writes the registered response.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	params := parseParams(body)
	command := params["Command"]

	s.mu.Lock()
	s.calls[command] = append(s.calls[command], params)
	route, ok := s.routes[command]
	var respBody string
	if ok {
		i := route.idx
		if i >= len(route.bodies) {
			i = len(route.bodies) - 1
		}
		respBody = route.bodies[i]
		route.idx++
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	if !ok {
		// Respond with an ERROR envelope (HTTP 200 so the SDK decodes it and maps
		// it to an *APIError) rather than failing from this goroutine, so the
		// consumer's call returns a legible error and their test fails cleanly.
		respBody = errorEnvelope(0, fmt.Sprintf("namecheaptest: no stub registered for command %q", command))
	}
	_, _ = io.WriteString(w, respBody)
}

// calledCommands returns the sorted set of commands that have been called, for
// diagnostics. It must be called without holding s.mu.
func (s *Server) calledCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.calls))
	for k := range s.calls {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// errorEnvelope synthesizes a Namecheap ERROR envelope carrying a single
// <Error Number="number">message</Error>, XML-escaping the message.
func errorEnvelope(number int, message string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(message))
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
	<Errors>
		<Error Number="%d">%s</Error>
	</Errors>
	<Warnings />
	<CommandResponse />
</ApiResponse>`, number, escaped.String())
}

// parseParams decodes a urlencoded request body into a flat parameter map,
// keeping the first value for any repeated key.
func parseParams(body []byte) map[string]string {
	params := make(map[string]string)
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return params
	}
	for k := range values {
		params[k] = values.Get(k)
	}
	return params
}

// cloneParams returns a shallow copy of a parameter map.
func cloneParams(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// matchesSubset reports whether got contains every key/value pair in want.
func matchesSubset(got, want map[string]string) bool {
	for k, v := range want {
		if gv, ok := got[k]; !ok || gv != v {
			return false
		}
	}
	return true
}
