package namecheap

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDomainsDNSSetHosts(t *testing.T) {
	fakeResponse := `
		<?xml version="1.0" encoding="utf-8"?>
		<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
			<Errors />
			<Warnings />
			<RequestedCommand>namecheap.domains.dns.sethosts</RequestedCommand>
			<CommandResponse Type="namecheap.domains.dns.setHosts">
				<DomainDNSSetHostsResult Domain="domain.net" EmailType="MX" IsSuccess="true">
					<Warnings />
				</DomainDNSSetHostsResult>
			</CommandResponse>
			<Server>PHX01SBAPIEXT05</Server>
			<GMTTimeDifference>--4:00</GMTTimeDifference>
			<ExecutionTime>0.854</ExecutionTime>
		</ApiResponse>
	`

	t.Run("request_command", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain: String("domain.net"),
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "namecheap.domains.dns.setHosts", sentBody.Get("Command"))
	})

	t.Run("request_data_correct_args_mapping", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain:    String("domain.net"),
			EmailType: String(EmailTypeForward),
			Flag:      UInt8(100),
			Tag:       String("issue"),
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, "domain", sentBody.Get("SLD"))
		assert.Equal(t, "net", sentBody.Get("TLD"))
		assert.Equal(t, "FWD", sentBody.Get("EmailType"))
		assert.Equal(t, "100", sentBody.Get("Flag"))
		assert.Equal(t, "issue", sentBody.Get("Tag"))
	})

	t.Run("request_data_correct_mx_records_mapping", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain:    String("domain.net"),
			EmailType: String("MX"),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeA),
					HostName:   String("@"),
					Address:    String("10.11.12.13"),
					TTL:        Int(1800),
				},
				{
					RecordType: String(RecordTypeMX),
					HostName:   String("mail"),
					Address:    String("super-mail.com"),
					TTL:        Int(1800),
					MXPref:     UInt8(10),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeA, sentBody.Get("RecordType1"))
		assert.Equal(t, "@", sentBody.Get("HostName1"))
		assert.Equal(t, "10.11.12.13", sentBody.Get("Address1"))
		assert.Equal(t, "1800", sentBody.Get("TTL1"))

		assert.Equal(t, RecordTypeMX, sentBody.Get("RecordType2"))
		assert.Equal(t, "mail", sentBody.Get("HostName2"))
		assert.Equal(t, "super-mail.com", sentBody.Get("Address2"))
		assert.Equal(t, "1800", sentBody.Get("TTL2"))
		assert.Equal(t, "10", sentBody.Get("MXPref2"))
	})

	t.Run("request_data_correct_mxe_records_mapping", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain:    String("domain.net"),
			EmailType: String(EmailTypeMXE),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeMXE),
					HostName:   String("mail"),
					Address:    String("10.11.12.13"),
					TTL:        Int(1800),
					MXPref:     UInt8(10),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeMXE, sentBody.Get("RecordType1"))
		assert.Equal(t, "mail", sentBody.Get("HostName1"))
		assert.Equal(t, "10.11.12.13", sentBody.Get("Address1"))
		assert.Equal(t, "1800", sentBody.Get("TTL1"))
		assert.Equal(t, "10", sentBody.Get("MXPref1"))
	})

	t.Run("request_data_correct_url_record", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain: String("domain.net"),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeURL),
					HostName:   String("redirect"),
					Address:    String("https://domain.com"),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeURL, sentBody.Get("RecordType1"))
		assert.Equal(t, "redirect", sentBody.Get("HostName1"))
		assert.Equal(t, "https://domain.com", sentBody.Get("Address1"))
	})

	t.Run("request_data_correct_url301_record", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain: String("domain.net"),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeURL301),
					HostName:   String("redirect"),
					Address:    String("https://domain.com"),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeURL301, sentBody.Get("RecordType1"))
		assert.Equal(t, "redirect", sentBody.Get("HostName1"))
		assert.Equal(t, "https://domain.com", sentBody.Get("Address1"))
	})

	t.Run("request_data_correct_frame_record", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain: String("domain.net"),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeFrame),
					HostName:   String("redirect"),
					Address:    String("https://domain.com"),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeFrame, sentBody.Get("RecordType1"))
		assert.Equal(t, "redirect", sentBody.Get("HostName1"))
		assert.Equal(t, "https://domain.com", sentBody.Get("Address1"))
	})

	t.Run("request_data_correct_CAA_iodef_record", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain: String("domain.net"),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeCAA),
					HostName:   String("@"),
					Address:    String("0 iodef http://domain.com"),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeCAA, sentBody.Get("RecordType1"))
		assert.Equal(t, "@", sentBody.Get("HostName1"))
		assert.Equal(t, "0 iodef http://domain.com", sentBody.Get("Address1"))
	})

	t.Run("request_data_correct_CAA_iodef_record_mailto", func(t *testing.T) {
		var sentBody url.Values

		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := ioutil.ReadAll(request.Body)
			query, _ := url.ParseQuery(string(body))
			sentBody = query
			_, _ = writer.Write([]byte(fakeResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		_, err := client.DomainsDNS.SetHosts(&DomainsDNSSetHostsArgs{
			Domain: String("domain.net"),
			Records: &[]DomainsDNSHostRecord{
				{
					RecordType: String(RecordTypeCAA),
					HostName:   String("@"),
					Address:    String("0 iodef mailto:hostmaster@domain.com"),
				},
			},
		})
		if err != nil {
			t.Fatal("Unable to get domains", err)
		}

		assert.Equal(t, RecordTypeCAA, sentBody.Get("RecordType1"))
		assert.Equal(t, "@", sentBody.Get("HostName1"))
		assert.Equal(t, "0 iodef mailto:hostmaster@domain.com", sentBody.Get("Address1"))
	})

	var errorCases = []struct {
		Name          string
		Args          *DomainsDNSSetHostsArgs
		ExpectedError string
	}{
		{
			Name: "request_data_error_incorrect_domain",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("dom"),
			},
			ExpectedError: "invalid domain: incorrect format",
		},
		{
			Name: "request_data_error_bad_email_type",
			Args: &DomainsDNSSetHostsArgs{
				Domain:    String("domain.net"),
				EmailType: String("BAD_TYPE"),
			},
			ExpectedError: "invalid EmailType value: BAD_TYPE",
		},
		{
			Name: "request_data_error_bad_tag",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Tag:    String("BAD_TAG"),
			},
			ExpectedError: "invalid Tag value: BAD_TAG",
		},
		{
			Name: "request_data_error_no_hostname",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String("CNAME"), Address: String("domain.com")},
				},
			},
			ExpectedError: "Records[0].HostName is required",
		},
		{
			Name: "request_data_error_no_recordtype",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{HostName: String("@"), Address: String("domain.com")},
				},
			},
			ExpectedError: "Records[0].RecordType is required",
		},
		{
			Name: "request_data_error_bad_recordtype",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String("BAD"), HostName: String("@"), Address: String("domain.com")},
				},
			},
			ExpectedError: "invalid Records[0].RecordType value: BAD",
		},
		{
			Name: "request_data_error_too_low_ttl",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeCNAME), HostName: String("@"), Address: String("domain.com"), TTL: Int(59)},
				},
			},
			ExpectedError: "invalid Records[0].TTL value: 59",
		},
		{
			Name: "request_data_error_too_big_ttl",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeCNAME), HostName: String("@"), Address: String("domain.com"), TTL: Int(60_001)},
				},
			},
			ExpectedError: "invalid Records[0].TTL value: 60001",
		},
		{
			Name: "request_data_error_no_address",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeCNAME), HostName: String("@")},
				},
			},
			ExpectedError: "Records[0].Address is required",
		},
		{
			Name: "request_data_error_email_type_mx_without_records",
			Args: &DomainsDNSSetHostsArgs{
				EmailType: String(EmailTypeMX),
				Domain:    String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeCNAME), HostName: String("@"), Address: String("domain.com"), TTL: Int(1800)},
				},
			},
			ExpectedError: "minimum 1 MX record required for MX EmailType",
		},
		{
			Name: "request_data_error_email_type_mxe_without_record",
			Args: &DomainsDNSSetHostsArgs{
				EmailType: String(EmailTypeMXE),
				Domain:    String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeCNAME), HostName: String("@"), Address: String("domain.com"), TTL: Int(1800)},
				},
			},
			ExpectedError: "one MXE record required for MXE EmailType",
		},
		{
			Name: "request_data_error_email_type_nil_with_mx",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeMX), HostName: String("mail"), Address: String("mail.domain.com"), MXPref: UInt8(10)},
				},
			},
			ExpectedError: "Records[0].RecordType MX is not allowed for EmailType=nil",
		},
		{
			Name: "request_data_error_email_type_fwd_with_mx",
			Args: &DomainsDNSSetHostsArgs{
				Domain:    String("domain.net"),
				EmailType: String(EmailTypeForward),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeMX), HostName: String("mail"), Address: String("mail.domain.com"), MXPref: UInt8(10)},
				},
			},
			ExpectedError: "Records[0].RecordType MX is not allowed for EmailType=FWD",
		},
		{
			Name: "request_data_error_email_type_nil_with_mxe",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeMXE), HostName: String("mail"), Address: String("10.11.12.13")},
				},
			},
			ExpectedError: "Records[0].RecordType MXE is not allowed for EmailType=nil",
		},
		{
			Name: "request_data_error_email_type_fwd_with_mxe",
			Args: &DomainsDNSSetHostsArgs{
				Domain:    String("domain.net"),
				EmailType: String(EmailTypeForward),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeMXE), HostName: String("mail"), Address: String("10.11.12.13")},
				},
			},
			ExpectedError: "Records[0].RecordType MXE is not allowed for EmailType=FWD",
		},
		{
			Name: "request_data_error_two_mxe_records",
			Args: &DomainsDNSSetHostsArgs{
				Domain:    String("domain.net"),
				EmailType: String(EmailTypeMXE),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeMXE), HostName: String("mail"), Address: String("10.11.12.13")},
					{RecordType: String(RecordTypeMXE), HostName: String("mail2"), Address: String("10.11.12.14")},
				},
			},
			ExpectedError: "one MXE record required for MXE EmailType",
		},
		{
			Name: "request_data_error_no_mxpref_for_mx_record",
			Args: &DomainsDNSSetHostsArgs{
				Domain:    String("domain.net"),
				EmailType: String(EmailTypeMX),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeMX), HostName: String("mail"), Address: String("mail.domain.com")},
				},
			},
			ExpectedError: "Records[0].MXPref is nil but required for MX record type",
		},
		{
			Name: "request_data_error_no_protocol_prefix_for_url_record",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeURL), HostName: String("mail"), Address: String("domain.com")},
				},
			},
			ExpectedError: `Records[0].Address "domain.com" must contain a protocol prefix for URL record`,
		},
		{
			Name: "request_data_error_no_protocol_prefix_for_url301_record",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeURL301), HostName: String("mail"), Address: String("domain.com")},
				},
			},
			ExpectedError: `Records[0].Address "domain.com" must contain a protocol prefix for URL301 record`,
		},
		{
			Name: "request_data_error_no_protocol_prefix_for_frame_record",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeFrame), HostName: String("mail"), Address: String("domain.com")},
				},
			},
			ExpectedError: `Records[0].Address "domain.com" must contain a protocol prefix for FRAME record`,
		},
		{
			Name: "request_data_error_no_protocol_prefix_for_caa_iodef_record",
			Args: &DomainsDNSSetHostsArgs{
				Domain: String("domain.net"),
				Records: &[]DomainsDNSHostRecord{
					{RecordType: String(RecordTypeCAA), HostName: String("@"), Address: String("0 iodef domain.com")},
				},
			},
			ExpectedError: `Records[0].Address "0 iodef domain.com" must contain a protocol prefix for CAA iodef record`,
		},
	}

	for _, errorCase := range errorCases {
		t.Run(errorCase.Name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_, _ = writer.Write([]byte(fakeResponse))
			}))
			defer mockServer.Close()

			client := setupClient(nil)
			client.BaseURL = mockServer.URL

			_, err := client.DomainsDNS.SetHosts(errorCase.Args)

			assert.EqualError(t, err, errorCase.ExpectedError)
		})
	}
}
