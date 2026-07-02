package namecheap

// UsersAddressService groups the namecheap.users.address.* API commands: the
// address-book CRUD used to store reusable registrant profiles
// (CreateWithContext, UpdateWithContext, DeleteWithContext, GetInfoWithContext,
// GetListWithContext, SetDefaultWithContext).
//
// An address-book entry stores the same logical fields as a domain ContactInfo,
// so an entry can feed the contact blocks of domains.create. Convert between the
// two shapes with ContactInfo.ToAddressDetails and UsersAddressDetails.ToContactInfo
// (and UsersAddressGetInfoResult.ToContactInfo for a fetched entry).
//
// Namecheap doc: https://www.namecheap.com/support/api/methods/users/address/
type UsersAddressService service

// UsersAddressDetails holds the mutable fields of an address-book entry shared by
// create and update. Field names and required/optional status follow the
// address.create request table in docs/namecheap-api-v2.md (lines 1347-1365); the
// same fields (plus AddressId) drive address.update (lines 1460-1477).
//
// Field naming differs from the domain ContactInfo shape: the address book uses
// "Zip" (not "PostalCode") and "Organization" (not "OrganizationName"), and it
// carries five fields ContactInfo has no counterpart for: AddressName, DefaultYN,
// StateProvinceChoice, PhoneExt and Fax. The adapter maps only the twelve shared
// logical fields (see ToContactInfo / ContactInfo.ToAddressDetails).
type UsersAddressDetails struct {
	// AddressName is the address label. Required.
	AddressName string
	// DefaultYN, when non-nil, sets (true -> "1") or clears (false -> "0") the
	// default flag. Nil omits the parameter. Optional.
	DefaultYN *bool
	// EmailAddress is the contact e-mail address. Required.
	EmailAddress string
	// FirstName is the contact's first name. Required.
	FirstName string
	// LastName is the contact's last name. Required.
	LastName string
	// JobTitle is the contact's job designation. Optional.
	JobTitle string
	// Organization is the contact's organization. Optional.
	Organization string
	// Address1 is the primary street address. Required.
	Address1 string
	// Address2 is the secondary street address. Optional.
	Address2 string
	// City is the contact's city. Required.
	City string
	// StateProvince is the state or province. Required.
	StateProvince string
	// StateProvinceChoice is the state/province choice. Required. Address-book
	// only; ContactInfo has no counterpart.
	StateProvinceChoice string
	// Zip is the postal/ZIP code. Required. (ContactInfo calls this PostalCode.)
	Zip string
	// Country is the two-letter country code. Required.
	Country string
	// Phone is the phone number in +NNN.NNNNNNNNNN format. Required.
	Phone string
	// PhoneExt is the phone extension. Optional.
	PhoneExt string
	// Fax is the fax number. Optional.
	Fax string
}

// requiredAddressFields returns the eleven always-required fields paired with
// their values (the doc marks these Required for both create and update). Optional
// fields (JobTitle, Organization, Address2, PhoneExt, Fax) and the DefaultYN flag
// are excluded.
func (d UsersAddressDetails) requiredAddressFields() []requiredContactField {
	return []requiredContactField{
		{"AddressName", d.AddressName},
		{"EmailAddress", d.EmailAddress},
		{"FirstName", d.FirstName},
		{"LastName", d.LastName},
		{"Address1", d.Address1},
		{"City", d.City},
		{"StateProvince", d.StateProvince},
		{"StateProvinceChoice", d.StateProvinceChoice},
		{"Zip", d.Zip},
		{"Country", d.Country},
		{"Phone", d.Phone},
	}
}

// missingFields returns the names of every required field that is empty, so a
// caller sees the complete list in a single error.
func (d UsersAddressDetails) missingFields() []string {
	req := d.requiredAddressFields()
	missing := make([]string, 0, len(req))
	for _, f := range req {
		if f.value == "" {
			missing = append(missing, f.suffix)
		}
	}
	return missing
}

// apply flattens the details into params. Required fields are always written
// (callers validate first); optional fields are written only when non-empty, and
// DefaultYN only when set.
func (d UsersAddressDetails) apply(params map[string]string) {
	params["AddressName"] = d.AddressName
	params["EmailAddress"] = d.EmailAddress
	params["FirstName"] = d.FirstName
	params["LastName"] = d.LastName
	params["Address1"] = d.Address1
	params["City"] = d.City
	params["StateProvince"] = d.StateProvince
	params["StateProvinceChoice"] = d.StateProvinceChoice
	params["Zip"] = d.Zip
	params["Country"] = d.Country
	params["Phone"] = d.Phone
	setIfNotEmpty(params, "JobTitle", d.JobTitle)
	setIfNotEmpty(params, "Organization", d.Organization)
	setIfNotEmpty(params, "Address2", d.Address2)
	setIfNotEmpty(params, "PhoneExt", d.PhoneExt)
	setIfNotEmpty(params, "Fax", d.Fax)
	if d.DefaultYN != nil {
		params["DefaultYN"] = oneZero(*d.DefaultYN)
	}
}

// ToContactInfo converts the twelve shared logical fields of an address-book
// entry into a domain ContactInfo, so a stored address can feed the contact
// blocks of domains.create. The name mappings are: Zip -> PostalCode and
// Organization -> OrganizationName; the other ten fields map by identical name.
// Address-book-only fields (AddressName, DefaultYN, StateProvinceChoice,
// PhoneExt, Fax) have no ContactInfo counterpart and are dropped.
func (d UsersAddressDetails) ToContactInfo() ContactInfo {
	return ContactInfo{
		FirstName:        d.FirstName,
		LastName:         d.LastName,
		Address1:         d.Address1,
		City:             d.City,
		StateProvince:    d.StateProvince,
		PostalCode:       d.Zip,
		Country:          d.Country,
		Phone:            d.Phone,
		EmailAddress:     d.EmailAddress,
		OrganizationName: d.Organization,
		JobTitle:         d.JobTitle,
		Address2:         d.Address2,
	}
}

// ToAddressDetails converts a domain ContactInfo into an address-book entry under
// the given addressName, applying the inverse mappings of ToContactInfo
// (PostalCode -> Zip, OrganizationName -> Organization). The address-book-only
// fields ContactInfo cannot supply (DefaultYN, StateProvinceChoice, PhoneExt,
// Fax) are left at their zero values for the caller to set; StateProvinceChoice
// in particular is required by the API, so set it before create/update.
func (c ContactInfo) ToAddressDetails(addressName string) UsersAddressDetails {
	return UsersAddressDetails{
		AddressName:   addressName,
		EmailAddress:  c.EmailAddress,
		FirstName:     c.FirstName,
		LastName:      c.LastName,
		JobTitle:      c.JobTitle,
		Organization:  c.OrganizationName,
		Address1:      c.Address1,
		Address2:      c.Address2,
		City:          c.City,
		StateProvince: c.StateProvince,
		Zip:           c.PostalCode,
		Country:       c.Country,
		Phone:         c.Phone,
	}
}

// oneZero maps a boolean onto Namecheap's "1"/"0" flag convention (used by
// DefaultYN).
func oneZero(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
