package namecheap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/hashicorp/go-cleanhttp"
)

var (
	debug = os.Getenv("DEBUG") != ""
)

const (
	namecheapApiUrl = "https://api.namecheap.com/xml.response"
	sandboxApiUrl   = "https://api.sandbox.namecheap.com/xml.response"
	getUserIPUrl    = "https://dynamicdns.park-your-domain.com/getip"
)

// New returns a Client instance by reading environment variables
func New() (*Client, error) {
	username := os.Getenv("NAMECHEAP_USERNAME")
	apiuser := os.Getenv("NAMECHEAP_API_USER")
	token := os.Getenv("NAMECHEAP_TOKEN")
	ip := os.Getenv("NAMECHEAP_IP")

	sbx := os.Getenv("NAMECHEAP_USE_SANDBOX")
	useSbx := sbx != "" && sbx != "false"

	return NewClient(username, apiuser, token, ip, useSbx)
}

// NewClient creates a Client instance from the provided configuration
// typically users call New() with environment variables set instead.
func NewClient(username string, apiuser string, token string, ip string, useSandbox bool) (*Client, error) {
	if username == "" || apiuser == "" || token == "" {
		return nil, fmt.Errorf("ERROR: missing configuration - username=%q, apiuser=%q, token=%d", username, apiuser, len(token))
	}

	// TODO(adam): parse `ip`, ipv4 only? is ipv6 allowed?
	client := Client{
		Token:    token,
		ApiUser:  apiuser,
		Username: username,
		Ip:       ip,
		URL:      namecheapApiUrl,
		Http:     cleanhttp.DefaultClient(),
	}

	if client.Ip == "" {
		ip, err := client.getUserIP()
		if err != nil {
			return nil, fmt.Errorf("ERROR: failed to get user IP: %v", err)
		}
		client.Ip = ip
	}

	if useSandbox {
		client.URL = sandboxApiUrl
	}

	return &client, nil
}

// Client provides a client to the Namecheap API
type Client struct {
	// Access Token
	Token string

	// ApiUser
	ApiUser string // TODO(adam): What's this for? difference with Username?

	// Username
	Username string

	// URL to the DO API to use
	URL string

	// IP that is whitelisted
	Ip string

	// HttpClient is the client to use. A client with
	// default values will be used if not provided.
	Http *http.Client
}

// Creates a new request with the params
func (c *Client) NewRequest(body map[string]string) (*http.Request, error) {
	u, err := url.Parse(c.URL)

	if err != nil {
		return nil, fmt.Errorf("Error parsing base URL: %s", err)
	}

	body["Username"] = c.Username
	body["ApiKey"] = c.Token
	body["ApiUser"] = c.ApiUser
	body["ClientIp"] = c.Ip

	rBody := c.encodeBody(body)

	if err != nil {
		return nil, fmt.Errorf("Error encoding request body: %s", err)
	}

	// Build the request
	req, err := http.NewRequest("POST", u.String(), bytes.NewBufferString(rBody))
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(rBody)))

	return req, nil

}

func (c *Client) decode(reader io.Reader, obj interface{}) error {
	if debug {
		bs, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}
		fmt.Printf("DEBUG: %q\n", string(bs))
		reader = bytes.NewReader(bs) // refill `reader`
	}

	decoder := xml.NewDecoder(reader)
	err := decoder.Decode(&obj)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) encodeBody(body map[string]string) string {
	data := url.Values{}
	for key, val := range body {
		data.Set(key, val)
	}
	return data.Encode()
}

func (c *Client) getUserIP() (string, error) {
	resp, err := c.Http.Get(getUserIPUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if debug {
		fmt.Printf("User IP: %s\n", string(ip))
	}

	return string(ip), nil
}
