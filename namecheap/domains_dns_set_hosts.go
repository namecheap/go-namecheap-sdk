package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	MinTTL int = 60
	MaxTTL int = 60000

	EmailTypeNone    = "NONE"
	EmailTypeMXE     = "MXE"
	EmailTypeMX      = "MX"
	EmailTypeForward = "FWD"
	EmailTypePrivate = "OX"
	EmailTypeGmail   = "GMAIL"

	RecordTypeA      = "A"
	RecordTypeAAAA   = "AAAA"
	RecordTypeAlias  = "ALIAS"
	RecordTypeCAA    = "CAA"
	RecordTypeCNAME  = "CNAME"
	RecordTypeMX     = "MX"
	RecordTypeMXE    = "MXE"
	RecordTypeNS     = "NS"
	RecordTypeTXT    = "TXT"
	RecordTypeURL    = "URL"
	RecordTypeURL301 = "URL301"
	RecordTypeFrame  = "FRAME"
)

var AllowedRecordTypeValues = []string{RecordTypeA, RecordTypeAAAA, RecordTypeAlias, RecordTypeCAA, RecordTypeCNAME, RecordTypeMX, RecordTypeMXE, RecordTypeNS, RecordTypeTXT, RecordTypeURL, RecordTypeURL301, RecordTypeFrame}
var AllowedEmailTypeValues = []string{EmailTypeNone, EmailTypeMXE, EmailTypeMX, EmailTypeForward, EmailTypePrivate, EmailTypeGmail}

var allowedTagValues = []string{"issue", "issuewild", "iodef"}
var validMailProtocolPrefix = regexp.MustCompile("mailto:.*@.*")
var validURLProtocolPrefix = regexp.MustCompile("[a-z]+://")

type DomainsDNSHostRecord struct {
	// Sub-domain/hostname to create the record for
	HostName *string
	// Possible values: A, AAAA, ALIAS, CAA, CNAME, MX, MXE, NS, TXT, URL, URL301, FRAME
	RecordType *string
	// Possible values are URL or ClientIp address. The value for this parameter is based on RecordType.
	Address *string
	// MX preference for host. Applicable for MX records only.
	MXPref *uint8
	// Time to live for all record types.Possible values: any value between 60 to 60000
	// Default Value: 1800 (if 0 value has been provided)
	TTL *int
	// AssociatedAppTitle is the read-only tag getHosts puts on a row created by a
	// Namecheap mail preset (for example "FWD" on the forwarding SPF TXT), and is
	// empty or nil on rows a user created. RecordFromDetailed copies it so callers
	// can tell preset-owned rows apart. setHosts has no input for it: it is never
	// sent, and an echoed row keeps its tag on the server.
	AssociatedAppTitle *string
}

type DomainsDNSSetHostsArgs struct {
	// Domain to setHosts
	Domain *string
	// DomainsDNSHostRecord list
	Records *[]DomainsDNSHostRecord
	// EmailType is the zone's mail mode. Possible values are MXE, MX, FWD, OX,
	// GMAIL or NONE.
	//
	// Leave it nil to keep the zone's current mode: the API reference marks the
	// parameter "Required: Yes", but the live API (sandbox and production,
	// 2026-10-08) preserves the mode when it is omitted. Set it only to change the
	// mode. Namecheap silently discards MX records unless the resulting mode is MX,
	// so SetHosts requires EmailType=MX whenever the records contain an MX record
	// (and MXE or MX for an MXE record), except for rows a mail preset created
	// (AssociatedAppTitle set), which are echoed as they are.
	//
	// See https://www.namecheap.com/support/knowledgebase/article.aspx/322/2237/how-can-i-set-up-mx-records-required-for-mail-service/
	EmailType *string
	// Is an unsigned integer between 0 and 255.
	// The flag value is an 8-bit number, the most significant bit of which indicates the criticality of understanding of a record by a CA.
	// It's recommended to use '0'
	// If nil provided, then this field is ignored
	Flag *uint8
	// A non-zero sequence of US-ASCII letters and numbers in lower case. The tag value can be one of the following values:
	// "issue" — specifies the certification authority that is authorized to issue a certificate for the domain name or subdomain record used in the title.
	// "issuewild" — specifies the certification authority that is allowed to issue a wildcard certificate for the domain name or subdomain record used in the title. The certificate applies to the domain name or subdomain directly and to all its subdomains.
	// "iodef" — specifies the e-mail address or URL (compliant with RFC 5070) a CA should use to notify a client if any issuance policy violation spotted by this CA.
	Tag *string
}

type DomainsDNSSetHostsResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *DomainsDNSSetHostsCommandResponse `xml:"CommandResponse"`
}

type DomainsDNSSetHostsCommandResponse struct {
	DomainDNSSetHostsResult *DomainDNSSetHostsResult `xml:"DomainDNSSetHostsResult"`
}

type DomainDNSSetHostsResult struct {
	Domain    *string `xml:"Domain,attr"`
	IsSuccess *bool   `xml:"IsSuccess,attr"`
}

func (d DomainDNSSetHostsResult) String() string {
	return fmt.Sprintf("{Domain: %v, IsSuccess: %v}", deref(d.Domain), deref(d.IsSuccess))
}

// SetHostsWithContext sets DNS host records settings for the requested domain.
// It replaces the whole zone: any existing row not in args.Records is deleted.
//
// A response with IsSuccess="false" is returned together with an error
// matching ErrCommandUnsuccessful; the zone is unchanged in that case.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/domains-dns/set-hosts/
func (dds *DomainsDNSService) SetHostsWithContext(ctx context.Context, args *DomainsDNSSetHostsArgs) (*DomainsDNSSetHostsCommandResponse, error) {
	var response DomainsDNSSetHostsResponse

	params := map[string]string{
		"Command": "namecheap.domains.dns.setHosts",
	}

	// validate input arguments
	err := validateDomainsDNSSetHostsArgs(args)
	if err != nil {
		return nil, err
	}

	// parse input arguments
	parsedArgsMap, err := parseDomainsDNSSetHostsArgs(args)
	if err != nil {
		return nil, err
	}

	// merge parsed arguments with params
	for k, v := range parsedArgsMap {
		params[k] = v
	}

	_, err = dds.client.DoXMLWithContext(ctx, params, &response)
	if err != nil {
		return nil, err
	}
	if r := response.CommandResponse; r != nil && r.DomainDNSSetHostsResult != nil &&
		r.DomainDNSSetHostsResult.IsSuccess != nil && !*r.DomainDNSSetHostsResult.IsSuccess {
		return r, fmt.Errorf("setHosts %s: %w", *args.Domain, ErrCommandUnsuccessful)
	}
	return response.CommandResponse, nil
}

// SetHosts sets DNS host records settings for the requested domain.
//
// Deprecated: SetHosts runs without a context. Use SetHostsWithContext. It is
// retained for backward compatibility and will be removed in v3.
func (dds *DomainsDNSService) SetHosts(args *DomainsDNSSetHostsArgs) (*DomainsDNSSetHostsCommandResponse, error) {
	return dds.SetHostsWithContext(context.Background(), args)
}

// nolint: gocyclo
func validateDomainsDNSSetHostsArgs(args *DomainsDNSSetHostsArgs) error {
	if args.EmailType != nil && !isValidEmailType(*args.EmailType) {
		return fmt.Errorf("invalid EmailType value: %s", *args.EmailType)
	}

	if args.Tag != nil && !isValidTagValue(*args.Tag) {
		return fmt.Errorf("invalid Tag value: %s", *args.Tag)
	}

	if args.Records != nil {
		for i, record := range *args.Records {
			if record.RecordType == nil {
				return fmt.Errorf("Records[%d].RecordType is required", i)
			}
			if !isValidRecordType(*record.RecordType) {
				return fmt.Errorf("invalid Records[%d].RecordType value: %s", i, *record.RecordType)
			}

			if record.HostName == nil {
				return fmt.Errorf("Records[%d].HostName is required", i)
			}

			if record.Address == nil {
				return fmt.Errorf("Records[%d].Address is required", i)
			}

			if record.TTL != nil && (*record.TTL < MinTTL || *record.TTL > MaxTTL) {
				return fmt.Errorf("invalid Records[%d].TTL value: %d", i, *record.TTL)
			}

			// A row a mail preset created is echoed as it is and belongs to the
			// preset, so the MX/MXE rules below apply to user rows only.
			presetRow := record.AssociatedAppTitle != nil && *record.AssociatedAppTitle != ""

			switch *record.RecordType {
			case "MX":
				if record.MXPref == nil {
					return fmt.Errorf("Records[%d].MXPref is nil but required for MX record type", i)
				}
				if !presetRow {
					if args.EmailType == nil {
						return fmt.Errorf("Records[%d].RecordType MX needs EmailType=MX: with EmailType omitted, Namecheap discards MX records unless the zone is already MX", i)
					}
					if *args.EmailType != EmailTypeMX {
						return fmt.Errorf("Records[%d].RecordType MX would be discarded by Namecheap under EmailType=%s; MX records are kept only under EmailType=MX", i, *args.EmailType)
					}
				}

			case "MXE":
				if !presetRow {
					if args.EmailType == nil {
						return fmt.Errorf("Records[%d].RecordType MXE needs EmailType=MXE or MX", i)
					}
					if !mxeAllowedUnder(*args.EmailType) {
						return fmt.Errorf("Records[%d].RecordType MXE is not allowed for EmailType=%s", i, *args.EmailType)
					}
				}

			case "URL", "URL301", "FRAME":
				if !validURLProtocolPrefix.MatchString(*record.Address) {
					return fmt.Errorf(`Records[%d].Address "%s" must contain a protocol prefix for %s record`, i, *record.Address, *record.RecordType)
				}

			case "CAA":
				if strings.Contains(*record.Address, "iodef") && !validMailProtocolPrefix.MatchString(*record.Address) && !validURLProtocolPrefix.MatchString(*record.Address) {
					return fmt.Errorf(`Records[%d].Address "%s" must contain a protocol prefix for %s iodef record`, i, *record.Address, *record.RecordType)
				}
			}

		}
	}

	return nil
}

// mxeAllowedUnder reports whether an MXE record may be written under an
// explicit emailType. MXE rows are verified to be kept under MXE and MX; under
// the other modes the behaviour is unverified, so they stay rejected.
func mxeAllowedUnder(emailType string) bool {
	return emailType == EmailTypeMXE || emailType == EmailTypeMX
}

func parseDomainsDNSSetHostsArgs(args *DomainsDNSSetHostsArgs) (map[string]string, error) {
	params := map[string]string{}

	parsedDomain, err := ParseDomain(*args.Domain)
	if err != nil {
		return nil, err
	}

	params["SLD"] = parsedDomain.SLD
	params["TLD"] = parsedDomain.TLD

	if args.EmailType != nil {
		params["EmailType"] = *args.EmailType
	}

	if args.Flag != nil {
		params["Flag"] = strconv.Itoa(int(*args.Flag))
	}

	if args.Tag != nil {
		params["Tag"] = *args.Tag
	}

	if args.Records != nil {
		for i, record := range *args.Records {
			recordIndexString := strconv.Itoa(i + 1)

			params["RecordType"+recordIndexString] = *record.RecordType

			if record.HostName != nil {
				params["HostName"+recordIndexString] = *record.HostName
			}

			if record.TTL != nil {
				params["TTL"+recordIndexString] = strconv.Itoa(*record.TTL)
			}

			if record.Address != nil {
				params["Address"+recordIndexString] = *record.Address
			}

			if record.MXPref != nil {
				params["MXPref"+recordIndexString] = strconv.Itoa(int(*record.MXPref))
			}

		}
	}

	return params, nil
}

func isValidEmailType(emailType string) bool {
	for _, value := range AllowedEmailTypeValues {
		if emailType == value {
			return true
		}
	}
	return false
}

func isValidTagValue(tag string) bool {
	for _, value := range allowedTagValues {
		if tag == value {
			return true
		}
	}
	return false
}

func isValidRecordType(recordType string) bool {
	for _, value := range AllowedRecordTypeValues {
		if recordType == value {
			return true
		}
	}
	return false
}
