package namecheap

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/namecheap/go-namecheap-sdk/v2/namecheap/internal/syncretry"
	"github.com/stretchr/testify/assert"
)

const (
	ncUserName = "user"
	ncAPIUser  = "user"
	ncAPIKey   = "token"
	ncClientIP = "10.10.10.10"
)

func setupClient(httpClient *http.Client) *Client {
	client := NewClient(&ClientOptions{
		UserName:   ncUserName,
		ApiUser:    ncAPIUser,
		ApiKey:     ncAPIKey,
		ClientIp:   ncClientIP,
		UseSandbox: false,
	})

	if httpClient != nil {
		client.http = httpClient
	}

	return client
}

func TestNewClient(t *testing.T) {
	t.Parallel()
	t.Run("client_credentials", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)

		assert.Equal(t, client.ClientOptions.UserName, ncUserName)
		assert.Equal(t, client.ClientOptions.ApiUser, ncAPIUser)
		assert.Equal(t, client.ClientOptions.ApiKey, ncAPIKey)
		assert.Equal(t, client.ClientOptions.ClientIp, ncClientIP)
	})

	t.Run("production_api_url", func(t *testing.T) {
		t.Parallel()
		client := NewClient(&ClientOptions{
			UserName:   ncUserName,
			ApiUser:    ncAPIUser,
			ApiKey:     ncAPIKey,
			ClientIp:   ncClientIP,
			UseSandbox: false,
		})

		assert.Equal(t, namecheapProductionAPIURL, client.BaseURL)
	})

	t.Run("sandbox_api_url", func(t *testing.T) {
		t.Parallel()
		client := NewClient(&ClientOptions{
			UserName:   ncUserName,
			ApiUser:    ncAPIUser,
			ApiKey:     ncAPIKey,
			ClientIp:   ncClientIP,
			UseSandbox: true,
		})

		assert.Equal(t, namecheapSandboxAPIURL, client.BaseURL)
	})
}

func TestNewRequest(t *testing.T) {
	t.Parallel()
	client := setupClient(nil)

	request, err := client.NewRequest(map[string]string{
		"Command": "command",
	})

	if err != nil {
		t.Fatal("Unable to create a request", err)
	}

	t.Run("correct_content_type", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, request.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
	})

	t.Run("correct_method_post", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, request.Method, "POST")
	})

	t.Run("correct_body", func(t *testing.T) {
		t.Parallel()
		body, err := io.ReadAll(request.Body)

		if err != nil {
			t.Fatal("Unable to read request body", err)
		}

		bodyString := strings.Split(string(body), "&")

		assert.Contains(t, bodyString, "ApiUser=user")
		assert.Contains(t, bodyString, "ApiKey=token")
		assert.Contains(t, bodyString, "ClientIp=10.10.10.10")
		assert.Contains(t, bodyString, "Username=user")
		assert.Contains(t, bodyString, "Command=command")
	})
}

func TestEncodeBody(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name string
		in   map[string]string
		out  string
	}{
		{
			name: "empty",
			in:   map[string]string{},
			out:  "",
		},
		{
			name: "one_param",
			in:   map[string]string{"param": "value"},
			out:  "param=value",
		},
		{
			name: "two_params",
			in:   map[string]string{"param1": "value1", "param2": "value2"},
			out:  "param1=value1&param2=value2",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, encodeBody(testCase.in), testCase.out)
		})
	}
}

func TestDecodeBody(t *testing.T) {
	t.Parallel()
	type Obj struct {
		String  string `xml:"String,attr"`
		Integer int    `xml:"Integer,attr"`
		Boolean bool   `xml:"Boolean,attr"`
	}

	expectedXML := "<Obj String=\"hello\" Integer=\"10\" Boolean=\"true\"></Obj>"

	obj := Obj{}

	err := decodeBody(strings.NewReader(expectedXML), &obj)

	if err != nil {
		log.Fatal("Unable to decode", err)
	}

	assert.Equal(t, obj.String, "hello")
	assert.Equal(t, obj.Integer, 10)
	assert.Equal(t, obj.Boolean, true)
}

func TestParseDomainErrorWrapping(t *testing.T) {
	t.Parallel()
	t.Run("publicsuffix_error_is_wrapped", func(t *testing.T) {
		t.Parallel()
		// "co.uk" passes regex validation but is a public suffix with no SLD,
		// so publicsuffix.Parse returns an error that ParseDomain wraps with %w.
		_, err := ParseDomain("co.uk")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid domain")
		// Verify the error is wrapped (unwrap returns the inner error)
		unwrapped := errors.Unwrap(err)
		assert.NotNil(t, unwrapped, "expected wrapped error to be unwrappable")
	})
}

func TestParseDomain(t *testing.T) {
	t.Parallel()
	successCases := []struct {
		Domain string
		TLD    string
		SLD    string
		TRD    string
	}{
		{
			Domain: "domain.com",
			TLD:    "com",
			SLD:    "domain",
			TRD:    "",
		},
		{
			Domain: "www.domain.com",
			TLD:    "com",
			SLD:    "domain",
			TRD:    "www",
		},
		{
			Domain: "dev2.domain.com",
			TLD:    "com",
			SLD:    "domain",
			TRD:    "dev2",
		},
		{
			Domain: "dev3.dev2.domain.com",
			TLD:    "com",
			SLD:    "domain",
			TRD:    "dev3.dev2",
		},
		{
			Domain: "dev2.domain.com",
			TLD:    "com",
			SLD:    "domain",
			TRD:    "dev2",
		},
		{
			Domain: "dev2.do-main.com",
			TLD:    "com",
			SLD:    "do-main",
			TRD:    "dev2",
		},
		{
			Domain: "www.capital.gov.ua",
			TLD:    "gov.ua",
			SLD:    "capital",
			TRD:    "www",
		},
		{
			Domain: "blog.government.co.uk",
			TLD:    "co.uk",
			SLD:    "government",
			TRD:    "blog",
		},
		{
			Domain: "an.name.co",
			TLD:    "co",
			SLD:    "name",
			TRD:    "an",
		},
	}

	errorCases := []struct {
		Domain        string
		ContainsError string
	}{
		{"www", "invalid domain: incorrect format"},
		{"", "invalid domain: incorrect format"},
		{".", "invalid domain: incorrect format"},
		{".www", "invalid domain: incorrect format"},
		{".domain.com", "invalid domain: incorrect format"},
		{"domain.com.", "invalid domain: incorrect format"},
		{"domain.com-ua", "invalid domain: incorrect format"},
		{"http://domain.ua", "invalid domain: incorrect format"},
		{"domain.ua/", "invalid domain: incorrect format"},
		{"do_main.ua", "invalid domain: incorrect format"},
	}

	for _, successCase := range successCases {
		t.Run("success_"+successCase.Domain, func(t *testing.T) {
			t.Parallel()
			parsedDomain, err := ParseDomain(successCase.Domain)
			if err != nil {
				t.Errorf("unable to parse domain %v", err)
				return
			}

			assert.Equal(t, successCase.TLD, parsedDomain.TLD)
			assert.Equal(t, successCase.SLD, parsedDomain.SLD)
			assert.Equal(t, successCase.TRD, parsedDomain.TRD)

		})
	}

	for _, errorCase := range errorCases {
		t.Run("error_"+errorCase.Domain, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDomain(errorCase.Domain)

			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), errorCase.ContainsError)
		})
	}

}

func TestNewRequestInvalidURL(t *testing.T) {
	t.Parallel()
	client := setupClient(nil)
	client.BaseURL = "://invalid"

	_, err := client.NewRequest(map[string]string{})
	assert.Error(t, err)
}

func TestDoXMLRetryExhausted(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer mockServer.Close()

	client := setupClient(nil)
	client.sr = syncretry.NewSyncRetry(&syncretry.Options{Delays: []int{1, 1}})
	client.BaseURL = mockServer.URL

	var result any
	_, err := client.DoXML(map[string]string{"Command": "test"}, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry limit exceeded")
}

func TestDoXMLDecodeFailure(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("not xml"))
	}))
	defer mockServer.Close()

	client := setupClient(nil)
	client.BaseURL = mockServer.URL

	var result any
	_, err := client.DoXML(map[string]string{"Command": "test"}, &result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse server response")
}

func TestDoXMLHTTPFailure(t *testing.T) {
	t.Parallel()
	mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`<?xml version="1.0"?><ApiResponse Status="OK"/>`))
	}))
	mockServer.Close()

	client := setupClient(nil)
	client.sr = syncretry.NewSyncRetry(&syncretry.Options{Delays: []int{}})
	client.BaseURL = mockServer.URL

	var result any
	_, err := client.DoXML(map[string]string{"Command": "test"}, &result)
	assert.Error(t, err)
}

func TestDoXMLWithContextCancelsInFlightRequest(t *testing.T) {
	t.Parallel()
	// The server blocks well past the point at which we cancel, so a prompt
	// return can only happen if cancellation aborts the in-flight request.
	mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = writer.Write([]byte(`<?xml version="1.0"?><ApiResponse Status="OK"/>`))
	}))
	defer mockServer.Close()

	client := setupClient(nil)
	client.BaseURL = mockServer.URL

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	var result any
	_, err := client.DoXMLWithContext(ctx, map[string]string{"Command": "test"}, &result)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected context.Canceled, got %v", err)
	assert.Less(t, elapsed, time.Second, "call should have returned promptly after cancellation")
}

func TestDoXMLWithContextCancelsRetrySleep(t *testing.T) {
	t.Parallel()
	// 405 forces the retry loop; a 30s inter-retry delay would dominate unless
	// the context deadline cancels the sleep.
	mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer mockServer.Close()

	client := setupClient(nil)
	client.sr = syncretry.NewSyncRetry(&syncretry.Options{Delays: []int{30}})
	client.BaseURL = mockServer.URL

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	var result any
	_, err := client.DoXMLWithContext(ctx, map[string]string{"Command": "test"}, &result)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected context.DeadlineExceeded, got %v", err)
	// Context errors propagate unwrapped, never rewritten to the retry-limit error.
	assert.NotContains(t, err.Error(), "API retry limit exceeded")
	assert.Less(t, elapsed, 5*time.Second, "call should have returned promptly after the deadline")
}

func TestDeprecatedWrapperDelegates(t *testing.T) {
	t.Parallel()
	// Regression: the deprecated, context-less wrapper must still work by
	// delegating to the ctx variant with context.Background().
	fakeResponse := `<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.getInfo</RequestedCommand>
			<CommandResponse Type="namecheap.domains.getInfo">
				<DomainGetInfoResult ID="123" DomainName="domain.com" IsPremium="false">
					<DnsDetails ProviderType="FreeDNS" IsUsingOurDNS="true" HostCount="0" />
				</DomainGetInfoResult>
			</CommandResponse>
		</ApiResponse>`

	mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(fakeResponse))
	}))
	defer mockServer.Close()

	client := setupClient(nil)
	client.BaseURL = mockServer.URL

	result, err := client.Domains.GetInfo("domain.com")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "domain.com", *result.DomainDNSGetListResult.DomainName)
}
