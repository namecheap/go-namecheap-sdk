// Package namecheap provides a Go client for the Namecheap API v1.
package namecheap

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/namecheap/go-namecheap-sdk/v2/namecheap/internal/syncretry"
	"github.com/weppos/publicsuffix-go/publicsuffix"
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
}

type Client struct {
	http   *http.Client
	common service
	sr     *syncretry.SyncRetry

	ClientOptions *ClientOptions
	BaseURL       string

	Domains    *DomainsService
	DomainsNS  *DomainsNSService
	DomainsDNS *DomainsDNSService
}

type service struct {
	client *Client
}

// NewClient returns a new Namecheap API Client
func NewClient(options *ClientOptions) *Client {
	client := &Client{
		ClientOptions: options,
		http:          cleanhttp.DefaultClient(),
		sr:            syncretry.NewSyncRetry(&syncretry.Options{Delays: []int{1, 5, 15, 30, 50}}),
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
// in-flight HTTP request, any pending inter-retry sleep, and waiting to enter
// the retry section. context.Canceled and context.DeadlineExceeded propagate
// to the caller unwrapped (they are never rewritten into a retry-limit error).
func (c *Client) DoXMLWithContext(ctx context.Context, body map[string]string, obj any) (*http.Response, error) {
	var requestResponse *http.Response
	err := c.sr.DoContext(ctx, func(ctx context.Context) error {
		request, err := c.NewRequestWithContext(ctx, body)
		if err != nil {
			return err
		}

		response, err := c.http.Do(request)
		if err != nil {
			return err
		}

		if response.StatusCode == 405 {
			response.Body.Close()
			return syncretry.ErrRetry
		}

		requestResponse = response
		err = decodeBody(response.Body, obj)
		response.Body.Close()
		return err
	})

	if err != nil && errors.Is(err, syncretry.ErrRetryAttempts) {
		return nil, fmt.Errorf("API retry limit exceeded")
	}

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

// decodeBody decodes the interface from received XML
func decodeBody(reader io.Reader, obj any) error {
	decoder := xml.NewDecoder(reader)
	err := decoder.Decode(&obj)
	if err != nil {
		return fmt.Errorf("unable to parse server response: %w", err)
	}
	return nil
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
