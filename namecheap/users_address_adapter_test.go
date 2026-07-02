package namecheap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContactInfoAddressRoundTrip proves the ContactInfo <-> address-book adapter
// preserves every shared logical field in both directions with no drift.
func TestContactInfoAddressRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("ContactInfo -> address -> ContactInfo", func(t *testing.T) {
		t.Parallel()
		original := validContactInfo()
		back := original.ToAddressDetails("Home").ToContactInfo()
		assert.Equal(t, original, back, "no ContactInfo field may drift through the address shape")
	})

	t.Run("address -> ContactInfo -> address", func(t *testing.T) {
		t.Parallel()
		// Only the twelve shared fields plus AddressName are populated; the
		// address-only extras (DefaultYN, StateProvinceChoice, PhoneExt, Fax) are
		// left zero so a full-equality round-trip is meaningful.
		original := UsersAddressDetails{
			AddressName:   "Home",
			EmailAddress:  "john@example.com",
			FirstName:     "John",
			LastName:      "Smith",
			JobTitle:      "Dev",
			Organization:  "NameCheap.com",
			Address1:      "8939 S.cross Blvd",
			Address2:      "Suite 600",
			City:          "Phoenix",
			StateProvince: "AZ",
			Zip:           "85284",
			Country:       "US",
			Phone:         "+1.6613102107",
		}
		back := original.ToContactInfo().ToAddressDetails(original.AddressName)
		assert.Equal(t, original, back, "no shared field may drift through the ContactInfo shape")
	})
}

// TestContactInfoAddressFieldMapping pins the two renamed correspondences that
// differ between the domain-contact and address-book shapes.
func TestContactInfoAddressFieldMapping(t *testing.T) {
	t.Parallel()

	c := validContactInfo()
	d := c.ToAddressDetails("Home")

	// PostalCode <-> Zip and OrganizationName <-> Organization are the renamed pair.
	assert.Equal(t, c.PostalCode, d.Zip)
	assert.Equal(t, c.OrganizationName, d.Organization)
	// The other ten shared fields map by identical name.
	assert.Equal(t, c.FirstName, d.FirstName)
	assert.Equal(t, c.EmailAddress, d.EmailAddress)
	assert.Equal(t, c.Address1, d.Address1)

	// And the inverse direction restores the ContactInfo names.
	got := d.ToContactInfo()
	assert.Equal(t, d.Zip, got.PostalCode)
	assert.Equal(t, d.Organization, got.OrganizationName)
}

// TestGetInfoResultToContactInfo proves a fetched address entry converts to a
// ContactInfo that can feed domains.create, applying the same field mapping.
func TestGetInfoResultToContactInfo(t *testing.T) {
	t.Parallel()

	result := &UsersAddressGetInfoResult{
		FirstName:     String("John"),
		LastName:      String("Smith"),
		Address1:      String("8939 S.cross Blvd"),
		Address2:      String("Suite 600"),
		City:          String("Phoenix"),
		StateProvince: String("AZ"),
		Zip:           String("85284"),
		Country:       String("US"),
		Phone:         String("+1.6613102107"),
		EmailAddress:  String("john@example.com"),
		Organization:  String("NameCheap.com"),
		JobTitle:      String("Dev"),
	}

	got := result.ToContactInfo()
	assert.Equal(t, validContactInfo(), got)
}

// TestGetInfoResultToContactInfoNil guards the nil receiver.
func TestGetInfoResultToContactInfoNil(t *testing.T) {
	t.Parallel()
	var result *UsersAddressGetInfoResult
	assert.Equal(t, ContactInfo{}, result.ToContactInfo())
}
