package namecheap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const addressCreateResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.create">
		<AddressCreateResult Success="true" AddressID="777" />
	</CommandResponse>
</ApiResponse>`

// addressGetInfoResponse echoes back exactly the fields validAddressDetails
// sends, so a Create -> GetInfo round-trip can assert field equality.
const addressGetInfoResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.address.getInfo">
		<GetAddressInfoResult>
			<AddressId>777</AddressId>
			<AddressName>Home</AddressName>
			<DefaultYN>0</DefaultYN>
			<EmailAddress>john@example.com</EmailAddress>
			<FirstName>John</FirstName>
			<LastName>Smith</LastName>
			<JobTitle>Dev</JobTitle>
			<Organization>NameCheap.com</Organization>
			<Address1>8939 S.cross Blvd</Address1>
			<Address2>Suite 600</Address2>
			<City>Phoenix</City>
			<StateProvince>AZ</StateProvince>
			<StateProvinceChoice>S</StateProvinceChoice>
			<Zip>85284</Zip>
			<Country>US</Country>
			<Phone>+1.6613102107</Phone>
			<PhoneExt>123</PhoneExt>
			<Fax>+1.6613102108</Fax>
		</GetAddressInfoResult>
	</CommandResponse>
</ApiResponse>`

func TestUsersAddressService_Create(t *testing.T) {
	t.Parallel()

	t.Run("success sends every field", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressCreateResponse, &sent)

		details := validAddressDetails()
		details.DefaultYN = Bool(true)

		resp, err := client.UsersAddress.CreateWithContext(context.Background(), details)
		mustNoError(t, err)

		assert.Equal(t, "namecheap.users.address.create", sent.Get("Command"))
		assert.Equal(t, "Home", sent.Get("AddressName"))
		assert.Equal(t, "1", sent.Get("DefaultYN"))
		assert.Equal(t, "John", sent.Get("FirstName"))
		assert.Equal(t, "85284", sent.Get("Zip"))
		assert.Equal(t, "S", sent.Get("StateProvinceChoice"))
		assert.Equal(t, "NameCheap.com", sent.Get("Organization"))

		mustNotNil(t, resp.AddressCreateResult)
		assert.True(t, *resp.AddressCreateResult.Success)
		assert.Equal(t, 777, *resp.AddressCreateResult.AddressID)
	})

	t.Run("default flag omitted when nil", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, addressCreateResponse, &sent)

		_, err := client.UsersAddress.CreateWithContext(context.Background(), validAddressDetails())
		mustNoError(t, err)
		_, hasDefault := sent["DefaultYN"]
		assert.False(t, hasDefault)
	})

	t.Run("nil details", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.UsersAddress.CreateWithContext(context.Background(), nil)
		assertInvalidArguments(t, err, "details")
	})

	t.Run("missing required fields reported all at once", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		details := validAddressDetails()
		details.AddressName = ""
		details.EmailAddress = ""
		details.StateProvinceChoice = ""

		_, err := client.UsersAddress.CreateWithContext(context.Background(), details)
		assertInvalidArguments(t, err, "AddressName", "EmailAddress", "StateProvinceChoice")
	})

	t.Run("api error mapped to APIError", func(t *testing.T) {
		t.Parallel()
		client := usersMockClient(t, apiErrorXML(4022336, "Failed to save user's address"), nil)
		_, err := client.UsersAddress.CreateWithContext(context.Background(), validAddressDetails())
		assertAPIError(t, err, 4022336)
	})
}

// TestUsersAddressService_CreateThenGetInfoEquality drives a Create followed by a
// GetInfo against a mock that routes by Command, and asserts every field written
// by Create round-trips back through GetInfo unchanged.
func TestUsersAddressService_CreateThenGetInfoEquality(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		q, _ := url.ParseQuery(string(raw))
		switch q.Get("Command") {
		case "namecheap.users.address.create":
			_, _ = w.Write([]byte(addressCreateResponse))
		case "namecheap.users.address.getInfo":
			_, _ = w.Write([]byte(addressGetInfoResponse))
		default:
			http.Error(w, "unexpected command", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := setupClient(nil)
	client.BaseURL = server.URL

	details := validAddressDetails()
	createResp, err := client.UsersAddress.CreateWithContext(context.Background(), details)
	mustNoError(t, err)
	addressID := *createResp.AddressCreateResult.AddressID

	infoResp, err := client.UsersAddress.GetInfoWithContext(context.Background(), addressID)
	mustNoError(t, err)
	got := infoResp.GetAddressInfoResult
	mustNotNil(t, got)

	assert.Equal(t, 777, *got.AddressID)
	assert.Equal(t, details.AddressName, *got.AddressName)
	assert.Equal(t, details.EmailAddress, *got.EmailAddress)
	assert.Equal(t, details.FirstName, *got.FirstName)
	assert.Equal(t, details.LastName, *got.LastName)
	assert.Equal(t, details.JobTitle, *got.JobTitle)
	assert.Equal(t, details.Organization, *got.Organization)
	assert.Equal(t, details.Address1, *got.Address1)
	assert.Equal(t, details.Address2, *got.Address2)
	assert.Equal(t, details.City, *got.City)
	assert.Equal(t, details.StateProvince, *got.StateProvince)
	assert.Equal(t, details.StateProvinceChoice, *got.StateProvinceChoice)
	assert.Equal(t, details.Zip, *got.Zip)
	assert.Equal(t, details.Country, *got.Country)
	assert.Equal(t, details.Phone, *got.Phone)
	assert.Equal(t, details.PhoneExt, *got.PhoneExt)
	assert.Equal(t, details.Fax, *got.Fax)
	assert.False(t, *got.DefaultYN)
}
