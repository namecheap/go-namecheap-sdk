package namecheap

// ContactInfo holds the details of a single domain contact (Registrant, Tech,
// Admin or AuxBilling). One definition serves three commands: it is the input
// for CreateWithContext and SetContactsWithContext (flattened into the request
// under a per-block prefix) and the parsed output of GetContactsWithContext.
//
// The nine leading fields are required by the Namecheap API for every contact
// block; the trailing three are optional. Field names and required/optional
// status follow docs/namecheap-api-v2.md (the create request table, lines
// 143-154). The struct tags map to the response element names used by
// getContacts.
type ContactInfo struct {
	// FirstName is the contact's first name. Required.
	FirstName string `xml:"FirstName"`
	// LastName is the contact's last name. Required.
	LastName string `xml:"LastName"`
	// Address1 is the primary street address. Required.
	Address1 string `xml:"Address1"`
	// City is the contact's city. Required.
	City string `xml:"City"`
	// StateProvince is the state or province. Required.
	StateProvince string `xml:"StateProvince"`
	// PostalCode is the postal/ZIP code. Required.
	PostalCode string `xml:"PostalCode"`
	// Country is the two-letter country code. Required.
	Country string `xml:"Country"`
	// Phone is the phone number in +NNN.NNNNNNNNNN format. Required.
	Phone string `xml:"Phone"`
	// EmailAddress is the contact e-mail address. Required.
	EmailAddress string `xml:"EmailAddress"`

	// OrganizationName is the contact's organization. Optional.
	OrganizationName string `xml:"OrganizationName"`
	// JobTitle is the contact's job title. Optional.
	JobTitle string `xml:"JobTitle"`
	// Address2 is the secondary street address. Optional.
	Address2 string `xml:"Address2"`
}

// requiredContactField pairs a field's Namecheap suffix with its current value,
// so validation can report every missing field without reflection.
type requiredContactField struct {
	suffix string
	value  string
}

// required returns the contact's nine required fields paired with their values.
func (c ContactInfo) required() []requiredContactField {
	return []requiredContactField{
		{"FirstName", c.FirstName},
		{"LastName", c.LastName},
		{"Address1", c.Address1},
		{"City", c.City},
		{"StateProvince", c.StateProvince},
		{"PostalCode", c.PostalCode},
		{"Country", c.Country},
		{"Phone", c.Phone},
		{"EmailAddress", c.EmailAddress},
	}
}

// missingRequiredFields returns the prefixed names of every required field that
// is empty (for example "RegistrantEmailAddress"), reporting all of them rather
// than only the first.
func (c ContactInfo) missingRequiredFields(prefix string) []string {
	req := c.required()
	missing := make([]string, 0, len(req))
	for _, f := range req {
		if f.value == "" {
			missing = append(missing, prefix+f.suffix)
		}
	}
	return missing
}

// apply flattens the contact into params under prefix, e.g. prefix "Registrant"
// writes "RegistrantFirstName", "RegistrantLastName", and so on. Required fields
// are always written (callers validate first); optional fields are written only
// when non-empty so an unset optional never sends a blank value.
func (c ContactInfo) apply(params map[string]string, prefix string) {
	params[prefix+"FirstName"] = c.FirstName
	params[prefix+"LastName"] = c.LastName
	params[prefix+"Address1"] = c.Address1
	params[prefix+"City"] = c.City
	params[prefix+"StateProvince"] = c.StateProvince
	params[prefix+"PostalCode"] = c.PostalCode
	params[prefix+"Country"] = c.Country
	params[prefix+"Phone"] = c.Phone
	params[prefix+"EmailAddress"] = c.EmailAddress
	setIfNotEmpty(params, prefix+"OrganizationName", c.OrganizationName)
	setIfNotEmpty(params, prefix+"JobTitle", c.JobTitle)
	setIfNotEmpty(params, prefix+"Address2", c.Address2)
}

// contactBlock names one of the four contact roles for validation and flattening.
type contactBlock struct {
	prefix  string
	contact ContactInfo
}

// contactBlocks returns the four contact blocks in the canonical order.
func contactBlocks(registrant, tech, admin, auxBilling ContactInfo) []contactBlock {
	return []contactBlock{
		{"Registrant", registrant},
		{"Tech", tech},
		{"Admin", admin},
		{"AuxBilling", auxBilling},
	}
}

// missingContactFields returns every missing required field across all four
// contact blocks, prefixed by role (e.g. "TechCity", "AdminPhone"). It reports
// all of them at once so a caller sees the complete list in a single error.
func missingContactFields(registrant, tech, admin, auxBilling ContactInfo) []string {
	blocks := contactBlocks(registrant, tech, admin, auxBilling)
	missing := make([]string, 0, len(blocks)*len(ContactInfo{}.required()))
	for _, b := range blocks {
		missing = append(missing, b.contact.missingRequiredFields(b.prefix)...)
	}
	return missing
}

// applyContacts flattens all four contact blocks into params.
func applyContacts(params map[string]string, registrant, tech, admin, auxBilling ContactInfo) {
	for _, b := range contactBlocks(registrant, tech, admin, auxBilling) {
		b.contact.apply(params, b.prefix)
	}
}

// setIfNotEmpty writes key=value into params only when value is non-empty.
func setIfNotEmpty(params map[string]string, key, value string) {
	if value != "" {
		params[key] = value
	}
}
