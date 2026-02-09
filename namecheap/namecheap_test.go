package namecheap

import (
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

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
	}, httpClient)

	return client
}

func TestNewClient(t *testing.T) {
	t.Run("client_credentials", func(t *testing.T) {
		client := setupClient(nil)

		assert.Equal(t, client.ClientOptions.UserName, ncUserName)
		assert.Equal(t, client.ClientOptions.ApiUser, ncAPIUser)
		assert.Equal(t, client.ClientOptions.ApiKey, ncAPIKey)
		assert.Equal(t, client.ClientOptions.ClientIp, ncClientIP)
	})

	t.Run("production_api_url", func(t *testing.T) {
		client := NewClient(&ClientOptions{
			UserName:   ncUserName,
			ApiUser:    ncAPIUser,
			ApiKey:     ncAPIKey,
			ClientIp:   ncClientIP,
			UseSandbox: false,
		}, nil)

		assert.Equal(t, namecheapProductionAPIURL, client.BaseURL)
	})

	t.Run("sandbox_api_url", func(t *testing.T) {
		client := NewClient(&ClientOptions{
			UserName:   ncUserName,
			ApiUser:    ncAPIUser,
			ApiKey:     ncAPIKey,
			ClientIp:   ncClientIP,
			UseSandbox: true,
		}, nil)

		assert.Equal(t, namecheapSandboxAPIURL, client.BaseURL)
	})
}

func TestNewRequest(t *testing.T) {
	client := setupClient(nil)

	request, err := client.NewRequest(map[string]string{
		"Command": "command",
	})

	if err != nil {
		t.Fatal("Unable to create a request", err)
	}

	t.Run("correct_content_type", func(t *testing.T) {
		assert.Equal(t, request.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
	})

	t.Run("correct_method_post", func(t *testing.T) {
		assert.Equal(t, request.Method, "POST")
	})

	t.Run("correct_body", func(t *testing.T) {
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
			assert.Equal(t, encodeBody(testCase.in), testCase.out)
		})
	}
}

func TestDecodeBody(t *testing.T) {
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

func TestParseDomain(t *testing.T) {
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
			_, err := ParseDomain(errorCase.Domain)

			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), errorCase.ContainsError)
		})
	}

}
