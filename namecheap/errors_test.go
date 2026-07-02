package namecheap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeNetError is a net.Error stand-in for exercising IsRetryable's transport
// branch without a real network stack.
type fakeNetError struct {
	timeout bool
}

func (e fakeNetError) Error() string   { return "fake net error" }
func (e fakeNetError) Timeout() bool   { return e.timeout }
func (e fakeNetError) Temporary() bool { return false }

// newMockClient returns a Client whose transport is redirected to a server that
// always writes body, together with the server's cleanup func.
func newMockClient(t *testing.T, body string) *Client {
	t.Helper()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(mockServer.Close)

	client := setupClient(nil)
	client.BaseURL = mockServer.URL
	return client
}

func TestAPIErrorError(t *testing.T) {
	t.Parallel()
	err := &APIError{Number: 2019166, Message: "Domain not found", Command: "namecheap.domains.getInfo"}
	// Legacy "<msg> (<number>)" format must be preserved verbatim.
	assert.Equal(t, "Domain not found (2019166)", err.Error())
}

func TestAPIErrorIsSameNumber(t *testing.T) {
	t.Parallel()
	err := &APIError{Number: 2019166, Message: "Domain not found"}
	assert.True(t, errors.Is(err, &APIError{Number: 2019166}))
	assert.False(t, errors.Is(err, &APIError{Number: 4011103}))
}

// sentinelCases is the grounded code->sentinel table, mirrored from errors.go.
var sentinelCases = []struct {
	name     string
	sentinel error
	codes    []int
}{
	{"ErrDomainNotFound", ErrDomainNotFound, []int{2019166}},
	{"ErrDomainNotAssociated", ErrDomainNotAssociated, []int{2016166, 3016166}},
	{"ErrDomainInvalid", ErrDomainInvalid, []int{2030166}},
	{"ErrTooManyDomains", ErrTooManyDomains, []int{2011169}},
	{"ErrPromotionCodeInvalid", ErrPromotionCodeInvalid, []int{2011170}},
	{"ErrOrderNotFound", ErrOrderNotFound, []int{2033409}},
	{"ErrAccessDenied", ErrAccessDenied, []int{4011103}},
	{"ErrServerError", ErrServerError, []int{3050900, 5019169, 5050169, 5050900}},
}

func TestSentinelCodeMap(t *testing.T) {
	t.Parallel()
	for _, tc := range sentinelCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, code := range tc.codes {
				assert.Truef(t, errors.Is(&APIError{Number: code}, tc.sentinel),
					"code %d should match sentinel %s", code, tc.name)
			}
			// A code outside the set must not match.
			assert.Falsef(t, errors.Is(&APIError{Number: 1}, tc.sentinel),
				"code 1 should not match sentinel %s", tc.name)
		})
	}
}

func TestSentinelCodesAreExclusive(t *testing.T) {
	t.Parallel()
	// A code belonging to one sentinel must not match any other sentinel,
	// guarding against accidental overlap in the grounded map.
	for _, owner := range sentinelCases {
		for _, code := range owner.codes {
			for _, other := range sentinelCases {
				if other.name == owner.name {
					continue
				}
				assert.Falsef(t, errors.Is(&APIError{Number: code}, other.sentinel),
					"code %d (owned by %s) should not match %s", code, owner.name, other.name)
			}
		}
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"server_error_3050900", &APIError{Number: 3050900}, true},
		{"server_error_5050900", &APIError{Number: 5050900}, true},
		{"server_error_5019169", &APIError{Number: 5019169}, true},
		{"server_error_5050169", &APIError{Number: 5050169}, true},
		{"not_found", &APIError{Number: 2019166}, false},
		{"validation", &APIError{Number: 2030166}, false},
		{"access_denied", &APIError{Number: 4011103}, false},
		{"wrapped_server_error", fmt.Errorf("call failed: %w", &APIError{Number: 3050900}), true},
		{"net_timeout", fakeNetError{timeout: true}, true},
		{"net_non_timeout", fakeNetError{timeout: false}, false},
		{"context_canceled", context.Canceled, false},
		{"context_deadline", context.DeadlineExceeded, false},
		// ctx errors can surface wrapped in a *url.Error whose Timeout() is true;
		// the ctx check must win so this stays non-retryable.
		{"url_error_wrapping_deadline", &url.Error{Op: "Post", URL: "x", Err: context.DeadlineExceeded}, false},
		{"parse_error", &ParseError{Err: io.EOF}, false},
		{"generic", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, IsRetryable(tc.err))
		})
	}
}

func TestSnippetCap(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc", snippet([]byte("abc")))
	big := strings.Repeat("x", maxSnippetBytes+100)
	assert.Len(t, snippet([]byte(big)), maxSnippetBytes)
}

func TestParseErrorThroughDoXML(t *testing.T) {
	t.Parallel()
	t.Run("garbage_body", func(t *testing.T) {
		t.Parallel()
		client := newMockClient(t, "totally not xml")

		var resp DomainsGetInfoResponse
		_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "namecheap.domains.getInfo"}, &resp)
		assert.Error(t, err)

		var parseErr *ParseError
		assert.True(t, errors.As(err, &parseErr))
		assert.Contains(t, err.Error(), "unable to parse server response")
		assert.NotNil(t, errors.Unwrap(err))
	})

	t.Run("snippet_is_bounded", func(t *testing.T) {
		t.Parallel()
		client := newMockClient(t, strings.Repeat("z", maxSnippetBytes+512))

		var resp DomainsGetInfoResponse
		_, err := client.DoXMLWithContext(context.Background(), map[string]string{"Command": "namecheap.domains.getInfo"}, &resp)

		var parseErr *ParseError
		assert.True(t, errors.As(err, &parseErr))
		assert.LessOrEqual(t, len(parseErr.Snippet), maxSnippetBytes)
	})
}

func TestDoXMLSingleTypedError(t *testing.T) {
	t.Parallel()
	body := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
			<Errors><Error Number="2019166">Domain not found</Error></Errors>
			<CommandResponse/>
		</ApiResponse>`
	client := newMockClient(t, body)

	// Through a real service method so Command is populated from its params.
	_, err := client.Domains.GetInfoWithContext(context.Background(), "example.com")
	assert.Error(t, err)

	var apiErr *APIError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 2019166, apiErr.Number)
	assert.Equal(t, "Domain not found", apiErr.Message)
	assert.Equal(t, "namecheap.domains.getInfo", apiErr.Command)
	assert.True(t, errors.Is(err, ErrDomainNotFound))
}

func TestDoXMLMultiErrorJoin(t *testing.T) {
	t.Parallel()
	body := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
			<Errors>
				<Error Number="2019166">Domain not found</Error>
				<Error Number="4011103">Access denied</Error>
			</Errors>
		</ApiResponse>`
	client := newMockClient(t, body)

	var resp DomainsGetInfoResponse
	_, err := client.DoXMLWithContext(context.Background(),
		map[string]string{"Command": "namecheap.domains.getInfo"}, &resp)
	assert.Error(t, err)

	// errors.Is finds each joined sentinel.
	assert.True(t, errors.Is(err, ErrDomainNotFound))
	assert.True(t, errors.Is(err, ErrAccessDenied))

	// errors.As reaches an *APIError, and it carries the command.
	var apiErr *APIError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "namecheap.domains.getInfo", apiErr.Command)
}

func TestServiceMethodsReturnTypedErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		number      string
		message     string
		wantCode    int
		wantCommand string
		sentinel    error
		call        func(*Client) error
	}{
		{
			name:        "domains_get_info",
			number:      "2019166",
			message:     "Domain not found",
			wantCode:    2019166,
			wantCommand: "namecheap.domains.getInfo",
			sentinel:    ErrDomainNotFound,
			call: func(c *Client) error {
				_, err := c.Domains.GetInfoWithContext(context.Background(), "example.com")
				return err
			},
		},
		{
			name:        "domains_check",
			number:      "2011169",
			message:     "Too many domains",
			wantCode:    2011169,
			wantCommand: "namecheap.domains.check",
			sentinel:    ErrTooManyDomains,
			call: func(c *Client) error {
				_, err := c.Domains.CheckWithContext(context.Background(), "example.com")
				return err
			},
		},
		{
			name:        "domains_ns_delete",
			number:      "2016166",
			message:     "Domain is not associated with your account",
			wantCode:    2016166,
			wantCommand: "namecheap.domains.ns.delete",
			sentinel:    ErrDomainNotAssociated,
			call: func(c *Client) error {
				_, err := c.DomainsNS.DeleteWithContext(context.Background(), "example", "com", "ns1.example.com")
				return err
			},
		},
		{
			name:        "domains_dns_set_default",
			number:      "2030166",
			message:     "Edit permission for domain is not supported",
			wantCode:    2030166,
			wantCommand: "namecheap.domains.dns.setDefault",
			sentinel:    ErrDomainInvalid,
			call: func(c *Client) error {
				_, err := c.DomainsDNS.SetDefaultWithContext(context.Background(), "example.com")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="%s">%s</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`, tc.number, tc.message)
			client := newMockClient(t, body)

			err := tc.call(client)
			assert.Error(t, err)

			var apiErr *APIError
			assert.True(t, errors.As(err, &apiErr), "expected an *APIError in the chain")
			assert.Equal(t, tc.wantCode, apiErr.Number)
			assert.Equal(t, tc.message, apiErr.Message)
			assert.Equal(t, tc.wantCommand, apiErr.Command)
			assert.True(t, errors.Is(err, tc.sentinel), "expected errors.Is to match the sentinel")
		})
	}
}
