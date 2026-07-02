package namecheap

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/weppos/publicsuffix-go/publicsuffix"
	"golang.org/x/time/rate"
)

var domainRegexp = regexp.MustCompile(`^([\-a-zA-Z0-9]+\.+){1,}[a-zA-Z0-9]+$`)

const (
	namecheapProductionAPIURL = "https://api.namecheap.com/xml.response"
	namecheapSandboxAPIURL    = "https://api.sandbox.namecheap.com/xml.response"
)

type ClientOptions struct {
	UserName   string
	ApiUser    string // nolint: stylecheck,revive
	ApiKey     string // nolint: stylecheck,revive
	ClientIp   string // nolint: stylecheck,revive
	UseSandbox bool

	// HTTPClient is the HTTP client used for every request. When nil, a
	// cleanhttp.DefaultClient() is used.
	HTTPClient *http.Client
	// Transport, when set, replaces the RoundTripper on the effective HTTP
	// client. Use it to inject middleware (retries, tracing, mocking) without
	// having to supply a whole *http.Client.
	Transport http.RoundTripper
	// UserAgent, when set, is appended (after a space) to the SDK's default
	// User-Agent on every request so support can identify the calling client.
	UserAgent string
	// RateLimit configures the client-side token-bucket limiter and optional
	// concurrency bound. When nil, the documented defaults apply (20 req/min).
	RateLimit *RateLimitOptions
	// Retry configures the exponential-backoff retry policy. When nil, the
	// documented defaults apply (4 attempts, 500ms base, 30s cap, 2m budget).
	Retry *RetryOptions

	// OnRequest, when set, is called once per outgoing HTTP attempt, immediately
	// before the request is sent and after any rate-limiter wait, with a
	// RequestInfo whose Params are already redacted (no credential is ever
	// exposed). It fires again for every retry with an increasing Attempt. A
	// panicking hook is recovered and logged (if a Logger is set), never crashing
	// the caller. Keep it fast and non-blocking; it runs on the request path.
	OnRequest func(RequestInfo)
	// OnResponse, when set, is called once per HTTP attempt after it completes,
	// with a ResponseInfo carrying the status, duration, outcome, error code and
	// whether a retry will follow. Same panic-safety and ordering guarantees as
	// OnRequest.
	OnResponse func(ResponseInfo)
	// Logger, when set, makes the SDK emit structured log/slog events on the same
	// request path as the hooks: request start (Debug), rate-limiter wait
	// (Debug), request completed (Info) and request failed/retrying (Warn).
	// Records carry command, attempt, duration, status and error_code attributes
	// and only ever include redacted parameters. It uses the stdlib log/slog, so
	// enabling it adds no dependency.
	Logger *slog.Logger
}

type Client struct {
	http      *http.Client
	common    service
	limiter   *rate.Limiter // nil when rate limiting is disabled
	sem       chan struct{} // nil when concurrency is unbounded
	retry     RetryOptions  // resolved retry policy (no zero fields)
	userAgent string

	// Observability surfaces, mirrored from ClientOptions for hot-path access.
	onRequest  func(RequestInfo)
	onResponse func(ResponseInfo)
	logger     *slog.Logger

	// Cumulative counters exposed via Stats, guarded by statsMu.
	statsMu         sync.Mutex
	statRequests    map[string]int64
	statErrors      map[int]int64
	statRetries     int64
	statLimiterWait time.Duration

	ClientOptions *ClientOptions
	BaseURL       string

	Domains         *DomainsService
	DomainsNS       *DomainsNSService
	DomainsDNS      *DomainsDNSService
	DomainsTransfer *DomainsTransferService

	Users        *UsersService
	UsersAddress *UsersAddressService

	SSL *SSLService
}

type service struct {
	client *Client
}

// NewClient returns a new Namecheap API Client.
//
// The client is safe for concurrent use. Requests flow through a client-side
// rate limiter, an optional concurrency gate, and a context-aware
// exponential-backoff retry policy; all three are configured via options and
// fall back to safe defaults when their option is nil. See RateLimitOptions and
// RetryOptions.
func NewClient(options *ClientOptions) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = cleanhttp.DefaultClient()
	}
	if options.Transport != nil {
		httpClient.Transport = options.Transport
	}

	client := &Client{
		ClientOptions: options,
		http:          httpClient,
		limiter:       newLimiter(options.RateLimit),
		sem:           newSemaphore(options.RateLimit),
		retry:         resolveRetry(options.Retry),
		userAgent:     resolveUserAgent(options.UserAgent),
		onRequest:     options.OnRequest,
		onResponse:    options.OnResponse,
		logger:        options.Logger,
		statRequests:  make(map[string]int64),
		statErrors:    make(map[int]int64),
	}

	if options.UseSandbox {
		client.BaseURL = namecheapSandboxAPIURL
	} else {
		client.BaseURL = namecheapProductionAPIURL
	}

	client.common.client = client
	client.Domains = (*DomainsService)(&client.common)
	client.DomainsDNS = (*DomainsDNSService)(&client.common)
	client.DomainsNS = (*DomainsNSService)(&client.common)
	client.DomainsTransfer = (*DomainsTransferService)(&client.common)
	client.Users = (*UsersService)(&client.common)
	client.UsersAddress = (*UsersAddressService)(&client.common)
	client.SSL = (*SSLService)(&client.common)

	return client
}

// NewRequestWithContext creates a new request with the params, bound to ctx.
// The returned *http.Request carries ctx, so cancelling it aborts the in-flight
// call when the request is executed.
func (c *Client) NewRequestWithContext(ctx context.Context, body map[string]string) (*http.Request, error) {
	u, err := url.Parse(c.BaseURL)

	if err != nil {
		return nil, fmt.Errorf("error parsing base URL: %w", err)
	}

	body["Username"] = c.ClientOptions.UserName
	body["ApiKey"] = c.ClientOptions.ApiKey
	body["ApiUser"] = c.ClientOptions.ApiUser
	body["ClientIp"] = c.ClientOptions.ClientIp

	rBody := encodeBody(body)

	// Build the request
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewBufferString(rBody))

	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(rBody)))
	req.Header.Set("User-Agent", c.userAgent)

	return req, nil
}

// NewRequest creates a new request with the params.
//
// Deprecated: NewRequest builds a request with no context, so the call cannot
// be cancelled or time-bounded. Use NewRequestWithContext. NewRequest is
// retained for backward compatibility and will be removed in v3.
func (c *Client) NewRequest(body map[string]string) (*http.Request, error) {
	return c.NewRequestWithContext(context.Background(), body)
}

// DoXMLWithContext performs the API call described by body, decoding the XML
// response into obj. The call is bound to ctx: cancelling ctx aborts the
// in-flight HTTP request, a pending rate-limit or concurrency wait, and any
// inter-retry backoff sleep. context.Canceled and context.DeadlineExceeded
// propagate to the caller unwrapped.
//
// The call flows through the client's rate limiter, concurrency gate, and
// exponential-backoff retry policy (see NewClient). An HTTP 405 — Namecheap's
// rate-limit signal — is retried transparently. A terminal retry failure wraps
// the last real error as "after N attempts: <err>", so errors.Is and errors.As
// still reach the underlying *APIError.
func (c *Client) DoXMLWithContext(ctx context.Context, body map[string]string, obj any) (*http.Response, error) {
	return c.doXML(ctx, body, obj, true)
}

// doXML is the shared engine behind DoXMLWithContext. It performs the API call
// described by body, decoding the XML response into obj, and threads idempotent
// into the retry driver.
//
// idempotent must be true for safely-repeatable calls (every read and every
// non-charge-bearing write) and false for charge-bearing, non-idempotent calls
// (domain create/renew/reactivate). A false value narrows the retry policy so an
// ambiguous transport/server failure — which may already have executed and
// charged the account — is never resent; only Namecheap's pre-execution HTTP 405
// rate-limit signal is retried. See shouldRetry.
func (c *Client) doXML(ctx context.Context, body map[string]string, obj any, idempotent bool) (*http.Response, error) {
	command := body["Command"]

	// The redacted view handed to observers must reflect exactly what goes on the
	// wire, including the credentials NewRequestWithContext injects per attempt.
	// Build it only when an observer is configured so the hot path stays clean.
	var observeParams map[string]string
	if c.observed() {
		observeParams = c.withCredentials(body)
	}

	var requestResponse *http.Response
	err := c.do(ctx, command, observeParams, idempotent, func(ctx context.Context) (int, error) {
		request, err := c.NewRequestWithContext(ctx, body)
		if err != nil {
			return 0, err
		}

		response, err := c.http.Do(request)
		if err != nil {
			return 0, err
		}

		if response.StatusCode == 405 {
			response.Body.Close()
			return response.StatusCode, errRetryStatus
		}

		data, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			return response.StatusCode, readErr
		}

		requestResponse = response

		if parseErr := decodeBody(bytes.NewReader(data), obj); parseErr != nil {
			return response.StatusCode, parseErr
		}

		return response.StatusCode, parseAPIError(data, body["Command"])
	})

	return requestResponse, err
}

// DoXML performs the API call described by body, decoding the XML response
// into obj.
//
// Deprecated: DoXML runs without a context, so the call cannot be cancelled or
// time-bounded. Use DoXMLWithContext. DoXML is retained for backward
// compatibility and will be removed in v3.
func (c *Client) DoXML(body map[string]string, obj any) (*http.Response, error) {
	return c.DoXMLWithContext(context.Background(), body, obj)
}

// apiResponseEnvelope is a minimal view of every Namecheap XML response used to
// extract typed errors centrally, independent of the per-command response type.
type apiResponseEnvelope struct {
	Status string `xml:"Status,attr"`
	Errors *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
}

// decodeBody reads and decodes the XML from reader into obj. A malformed body
// yields a *ParseError carrying a bounded snippet of the raw response and the
// underlying decode error (its message keeps the legacy
// "unable to parse server response:" prefix).
func decodeBody(reader io.Reader, obj any) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return &ParseError{Snippet: snippet(data), Err: err}
	}
	if err := xml.Unmarshal(data, obj); err != nil {
		return &ParseError{Snippet: snippet(data), Err: err}
	}
	return nil
}

// parseAPIError extracts a typed API error from an already-decoded response
// body. It returns a single *APIError when the response carries exactly one
// <Error>, an errors.Join of *APIError values when it carries several, and nil
// when the response is not an error. command is threaded onto each *APIError.
func parseAPIError(data []byte, command string) error {
	var env apiResponseEnvelope
	if err := xml.Unmarshal(data, &env); err != nil {
		// A body that decoded for the caller's type but not for the envelope is
		// treated as "no API error"; a genuinely malformed body is already
		// reported as a *ParseError by decodeBody.
		return nil
	}

	var entries []struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	}
	if env.Errors != nil {
		entries = *env.Errors
	}

	if len(entries) == 0 {
		if env.Status == "ERROR" {
			return &APIError{Command: command, Message: "API returned an error status"}
		}
		return nil
	}

	apiErrors := make([]error, 0, len(entries))
	for _, e := range entries {
		message := ""
		if e.Message != nil {
			message = *e.Message
		}
		apiErrors = append(apiErrors, &APIError{
			Number:  atoiOrZero(e.Number),
			Message: message,
			Command: command,
		})
	}

	if len(apiErrors) == 1 {
		return apiErrors[0]
	}
	return errors.Join(apiErrors...)
}

// encodeBody converts the map into query string
func encodeBody(body map[string]string) string {
	data := url.Values{}
	for key, val := range body {
		data.Set(key, val)
	}
	return data.Encode()
}

// ParseDomain is a wrapper around publicsuffix.Parse to throw the correct error
func ParseDomain(domain string) (*publicsuffix.DomainName, error) {
	if !domainRegexp.MatchString(domain) {
		return nil, fmt.Errorf("invalid domain: incorrect format")
	}

	parsedDomain, err := publicsuffix.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	return parsedDomain, nil
}

// Bool is a helper routine that allocates a new bool value
// to store v and returns a pointer to it.
func Bool(v bool) *bool { return &v }

// Int is a helper routine that allocates a new int value
// to store v and returns a pointer to it.
func Int(v int) *int { return &v }

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
func String(v string) *string { return &v }

// UInt8 is a helper routine that allocates a new uint8 value
// to store v and returns a pointer to it.
func UInt8(v uint8) *uint8 { return &v }
