package namecheap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// privacyRecorder records, per wire Command, how many times it was called and the
// last request parameters it received. It backs the EnsureEnabled state-machine
// mock sequence so a test can assert both the branch taken (call counts) and the
// wire parameters (allot DomainName, enable ForwardedToEmail).
type privacyRecorder struct {
	mu    sync.Mutex
	calls map[string]int
	last  map[string]url.Values
}

func newPrivacyRecorder() *privacyRecorder {
	return &privacyRecorder{calls: map[string]int{}, last: map[string]url.Values{}}
}

func (r *privacyRecorder) record(cmd string, v url.Values) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[cmd]++
	r.last[cmd] = v
}

func (r *privacyRecorder) count(cmd string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[cmd]
}

func (r *privacyRecorder) lastOf(cmd string) url.Values {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last[cmd]
}

// privacyGetListXML builds a getList OK response from raw <Whoisguard/> rows.
func privacyGetListXML(rows string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<CommandResponse Type="namecheap.whoisguard.getlist">
				<WhoisguardGetListResult>%s</WhoisguardGetListResult>
				<Paging><TotalItems>1</TotalItems><CurrentPage>1</CurrentPage><PageSize>100</PageSize></Paging>
			</CommandResponse>
		</ApiResponse>`, rows)
}

// ensureEnabledDispatchServer returns a mock server that dispatches on the wire
// Command: getList returns getListBody; allot and enable return canned OK bodies.
// Every call is recorded so the state machine's path is assertable.
func ensureEnabledDispatchServer(t *testing.T, rec *privacyRecorder, getListBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		cmd := vals.Get("Command")
		rec.record(cmd, vals)
		switch cmd {
		case "namecheap.whoisguard.getlist":
			_, _ = w.Write([]byte(getListBody))
		case "namecheap.whoisguard.allot":
			_, _ = w.Write([]byte(domainPrivacyAllotOKResponse))
		case "namecheap.whoisguard.enable":
			_, _ = w.Write([]byte(domainPrivacyEnableOKResponse))
		default:
			t.Errorf("unexpected command on the wire: %q", cmd)
			_, _ = w.Write([]byte(apiErrorXML(9999, "unexpected command")))
		}
	}))
}

func TestDomainPrivacyService_EnsureEnabled(t *testing.T) {
	t.Parallel()

	const (
		domain = "example.com"
		fwd    = "[email protected]"
	)

	cases := []struct {
		name         string
		rows         string
		wantAction   EnsureEnabledAction
		wantID       int
		wantAllot    int // expected allot call count
		wantEnable   int // expected enable call count
		wantErrIsNFS bool
	}{
		{
			name:       "unallotted_free_gets_allotted_then_enabled",
			rows:       `<Whoisguard ID="1001" DomainName="" Created="01/01/2024" Expires="01/01/2025" Status="FREE" />`,
			wantAction: EnsureEnabledAllottedAndEnabled,
			wantID:     1001,
			wantAllot:  1,
			wantEnable: 1,
		},
		{
			name:       "allotted_but_disabled_gets_enabled",
			rows:       `<Whoisguard ID="2002" DomainName="example.com" Created="01/01/2024" Expires="01/01/2025" Status="DISABLED" />`,
			wantAction: EnsureEnabledEnabled,
			wantID:     2002,
			wantAllot:  0,
			wantEnable: 1,
		},
		{
			name:       "already_enabled_is_noop",
			rows:       `<Whoisguard ID="3003" DomainName="example.com" Created="01/01/2024" Expires="01/01/2025" Status="ENABLED" />`,
			wantAction: EnsureEnabledAlreadyEnabled,
			wantID:     3003,
			wantAllot:  0,
			wantEnable: 0,
		},
		{
			name:         "no_free_subscription_returns_typed_error",
			rows:         `<Whoisguard ID="4004" DomainName="other.com" Created="01/01/2024" Expires="01/01/2025" Status="ENABLED" />`,
			wantErrIsNFS: true,
			wantAllot:    0,
			wantEnable:   0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := newPrivacyRecorder()
			server := ensureEnabledDispatchServer(t, rec, privacyGetListXML(tc.rows))
			defer server.Close()

			client := setupClient(nil)
			client.BaseURL = server.URL

			res, err := client.DomainPrivacy.EnsureEnabledWithContext(context.Background(), domain, fwd)

			if tc.wantErrIsNFS {
				assert.Nil(t, res)
				assert.True(t, errors.Is(err, ErrNoFreePrivacySubscription), "want ErrNoFreePrivacySubscription, got %v", err)
			} else {
				assert.NoError(t, err)
				if assert.NotNil(t, res) {
					assert.Equal(t, tc.wantAction, res.Action)
					assert.Equal(t, tc.wantID, res.PrivacyID)
					assert.Equal(t, domain, res.Domain)
				}
			}

			// getList is always read exactly once.
			assert.Equal(t, 1, rec.count("namecheap.whoisguard.getlist"))
			assert.Equal(t, tc.wantAllot, rec.count("namecheap.whoisguard.allot"), "allot call count")
			assert.Equal(t, tc.wantEnable, rec.count("namecheap.whoisguard.enable"), "enable call count")

			// getList must request ListType=ALL and the documented max page size.
			gl := rec.lastOf("namecheap.whoisguard.getlist")
			assert.Equal(t, "ALL", gl.Get("ListType"))
			assert.Equal(t, "100", gl.Get("PageSize"))

			// When allot ran, it must carry the domain; when enable ran, the fwd email.
			if tc.wantAllot > 0 {
				al := rec.lastOf("namecheap.whoisguard.allot")
				assert.Equal(t, domain, al.Get("DomainName"))
				assert.Equal(t, fmt.Sprintf("%d", tc.wantID), al.Get("WhoisguardID"))
			}
			if tc.wantEnable > 0 {
				en := rec.lastOf("namecheap.whoisguard.enable")
				assert.Equal(t, fwd, en.Get("ForwardedToEmail"))
				assert.Equal(t, fmt.Sprintf("%d", tc.wantID), en.Get("WhoisguardID"))
			}
		})
	}
}

func TestDomainPrivacyService_EnsureEnabled_Validation(t *testing.T) {
	t.Parallel()

	client := setupClient(nil)
	client.BaseURL = "http://127.0.0.1:0"

	_, err := client.DomainPrivacy.EnsureEnabledWithContext(context.Background(), "", "")
	var argErr *InvalidArgumentsError
	if assert.True(t, errors.As(err, &argErr)) {
		assert.Contains(t, argErr.Fields, "DomainName")
		assert.Contains(t, argErr.Fields, "ForwardedToEmail")
	}
}

// TestDomainPrivacyService_EnsureEnabled_GetListError proves a getList failure
// short-circuits the state machine before any mutation.
func TestDomainPrivacyService_EnsureEnabled_GetListError(t *testing.T) {
	t.Parallel()
	rec := newPrivacyRecorder()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		rec.record(vals.Get("Command"), vals)
		_, _ = w.Write([]byte(apiErrorXML(2019166, "Domain not found")))
	}))
	defer server.Close()

	client := setupClient(nil)
	client.BaseURL = server.URL

	res, err := client.DomainPrivacy.EnsureEnabledWithContext(context.Background(), "example.com", "[email protected]")
	assert.Nil(t, res)
	var apiErr *APIError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 0, rec.count("namecheap.whoisguard.allot"))
	assert.Equal(t, 0, rec.count("namecheap.whoisguard.enable"))
}
