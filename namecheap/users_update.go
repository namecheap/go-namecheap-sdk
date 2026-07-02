package namecheap

import (
	"context"
	"encoding/xml"
)

// UsersUpdateResponse is the raw envelope for namecheap.users.update.
type UsersUpdateResponse struct {
	XMLName *xml.Name `xml:"ApiResponse"`
	Errors  *[]struct {
		Message *string `xml:",chardata"`
		Number  *string `xml:"Number,attr"`
	} `xml:"Errors>Error"`
	CommandResponse *UsersUpdateCommandResponse `xml:"CommandResponse"`
}

// UsersUpdateCommandResponse wraps the update result. The doc lists no response
// table for users.update (docs/namecheap-api-v2.md lines 1186-1210); the result
// element carries a Success flag on the wire.
type UsersUpdateCommandResponse struct {
	UserUpdateResult *UsersUpdateResult `xml:"UserUpdateResult"`
}

// UsersUpdateResult is the outcome of an account update.
type UsersUpdateResult struct {
	// Success reports whether the account was updated.
	Success *bool `xml:"Success,attr"`
}

// UsersUpdateArgs are the arguments for UpdateWithContext. Field names and
// required/optional status follow the update request table in
// docs/namecheap-api-v2.md (lines 1196-1209).
//
// Note the field naming differs from the domain ContactInfo shape: this command
// uses "Zip" (not "PostalCode"), "Organization" (not "OrganizationName") and adds
// PhoneExt/Fax. It is the account owner's own profile, not a per-domain contact,
// so no ContactInfo adapter is provided for it.
type UsersUpdateArgs struct {
	// FirstName is the account holder's first name. Required.
	FirstName string
	// LastName is the account holder's last name. Required.
	LastName string
	// JobTitle is the job designation. Optional.
	JobTitle string
	// Organization is the organization name. Optional.
	Organization string
	// Address1 is the primary street address. Required.
	Address1 string
	// Address2 is the secondary street address. Optional.
	Address2 string
	// City is the city. Required.
	City string
	// StateProvince is the state or province. Required.
	StateProvince string
	// Zip is the postal/ZIP code. Required.
	Zip string
	// Country is the two-letter country code. Required.
	Country string
	// EmailAddress is the account e-mail address. Required.
	EmailAddress string
	// Phone is the phone number in +NNN.NNNNNNNNNN format. Required.
	Phone string
	// PhoneExt is the phone extension. Optional.
	PhoneExt string
	// Fax is the fax number in +NNN.NNNNNNNNNN format. Optional.
	Fax string
}

// UpdateWithContext updates the account contact information.
//
// It is a non-idempotent account write but not charge-bearing; it uses the
// standard idempotent transport.
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/update/
func (us *UsersService) UpdateWithContext(ctx context.Context, args *UsersUpdateArgs) (*UsersUpdateCommandResponse, error) {
	if args == nil {
		return nil, &InvalidArgumentsError{Fields: []string{"args"}, Reason: "args must not be nil"}
	}
	if err := args.validate(); err != nil {
		return nil, err
	}

	var response UsersUpdateResponse
	_, err := us.client.DoXMLWithContext(ctx, args.params(), &response)
	if err != nil {
		return nil, err
	}
	return response.CommandResponse, nil
}

// validate reports every missing required field at once.
func (a *UsersUpdateArgs) validate() error {
	required := []requiredContactField{
		{"FirstName", a.FirstName},
		{"LastName", a.LastName},
		{"Address1", a.Address1},
		{"City", a.City},
		{"StateProvince", a.StateProvince},
		{"Zip", a.Zip},
		{"Country", a.Country},
		{"EmailAddress", a.EmailAddress},
		{"Phone", a.Phone},
	}
	missing := make([]string, 0, len(required))
	for _, f := range required {
		if f.value == "" {
			missing = append(missing, f.suffix)
		}
	}
	if len(missing) > 0 {
		return &InvalidArgumentsError{Fields: missing}
	}
	return nil
}

// params flattens the validated args into the request map.
func (a *UsersUpdateArgs) params() map[string]string {
	params := map[string]string{
		"Command":       "namecheap.users.update",
		"FirstName":     a.FirstName,
		"LastName":      a.LastName,
		"Address1":      a.Address1,
		"City":          a.City,
		"StateProvince": a.StateProvince,
		"Zip":           a.Zip,
		"Country":       a.Country,
		"EmailAddress":  a.EmailAddress,
		"Phone":         a.Phone,
	}
	setIfNotEmpty(params, "JobTitle", a.JobTitle)
	setIfNotEmpty(params, "Organization", a.Organization)
	setIfNotEmpty(params, "Address2", a.Address2)
	setIfNotEmpty(params, "PhoneExt", a.PhoneExt)
	setIfNotEmpty(params, "Fax", a.Fax)
	return params
}
