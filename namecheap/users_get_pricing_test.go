package namecheap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pricingDomainResponse is a realistic captured-style DOMAIN price sheet. The
// register tiers for com/net/org are crafted to exercise EffectivePrice
// precedence: com has a promo Price, net falls through to YourPrice (Price zero),
// org falls through to RegularPrice (Price empty, YourPrice zero). Extra
// attributes (Currency, PricingType, ...) mirror the live API and must be ignored.
const pricingDomainResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<RequestedCommand>namecheap.users.getPricing</RequestedCommand>
	<CommandResponse Type="namecheap.users.getPricing">
		<UserGetPricingResult>
			<ProductType Name="domains">
				<ProductCategory Name="register">
					<Product Name="com">
						<Price Duration="1" DurationType="YEAR" Price="8.88" PricingType="MULTIPLE" RegularPrice="10.87" RegularPriceType="MULTIPLE" YourPrice="9.99" YourPriceType="MULTIPLE" Currency="USD" />
						<Price Duration="2" DurationType="YEAR" Price="17.76" RegularPrice="21.74" YourPrice="19.98" Currency="USD" />
					</Product>
					<Product Name="net">
						<Price Duration="1" DurationType="YEAR" Price="0.0" RegularPrice="12.00" YourPrice="10.50" Currency="USD" />
					</Product>
					<Product Name="org">
						<Price Duration="1" DurationType="YEAR" Price="" RegularPrice="9.18" YourPrice="0.00" Currency="USD" />
					</Product>
				</ProductCategory>
				<ProductCategory Name="renew">
					<Product Name="com">
						<Price Duration="1" DurationType="YEAR" Price="11.00" RegularPrice="12.88" YourPrice="11.50" Currency="USD" />
					</Product>
				</ProductCategory>
			</ProductType>
		</UserGetPricingResult>
	</CommandResponse>
	<Server>PHX01SBAPIEXT06</Server>
	<GMTTimeDifference>--4:00</GMTTimeDifference>
	<ExecutionTime>2.345</ExecutionTime>
</ApiResponse>`

const pricingSSLResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.getPricing">
		<UserGetPricingResult>
			<ProductType Name="ssl">
				<ProductCategory Name="purchase">
					<Product Name="positivessl">
						<Price Duration="1" DurationType="YEAR" Price="9.00" RegularPrice="9.00" YourPrice="9.00" Currency="USD" />
						<Price Duration="2" DurationType="YEAR" Price="16.00" RegularPrice="18.00" YourPrice="16.00" Currency="USD" />
					</Product>
				</ProductCategory>
			</ProductType>
		</UserGetPricingResult>
	</CommandResponse>
</ApiResponse>`

const pricingWhoisguardResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
	<Errors />
	<CommandResponse Type="namecheap.users.getPricing">
		<UserGetPricingResult>
			<ProductType Name="whoisguard">
				<ProductCategory Name="renew">
					<Product Name="whoisguard">
						<Price Duration="1" DurationType="YEAR" Price="2.88" RegularPrice="2.88" YourPrice="2.88" Currency="USD" />
					</Product>
				</ProductCategory>
			</ProductType>
		</UserGetPricingResult>
	</CommandResponse>
</ApiResponse>`

func TestUsersService_GetPricing_Domain(t *testing.T) {
	t.Parallel()

	var sent url.Values
	client := usersMockClient(t, pricingDomainResponse, &sent)

	resp, err := client.Users.GetPricingWithContext(context.Background(), &UsersGetPricingArgs{
		ProductType:     String("DOMAIN"),
		ProductCategory: String("REGISTER"),
		ActionName:      String("REGISTER"),
		ProductName:     String("com"),
	})
	mustNoError(t, err)

	// Request carries the command and every provided narrowing parameter.
	assert.Equal(t, "namecheap.users.getPricing", sent.Get("Command"))
	assert.Equal(t, "DOMAIN", sent.Get("ProductType"))
	assert.Equal(t, "REGISTER", sent.Get("ProductCategory"))
	assert.Equal(t, "REGISTER", sent.Get("ActionName"))
	assert.Equal(t, "com", sent.Get("ProductName"))

	// The nested tree is fully navigable and keeps the doc's element/attr names.
	result := resp.UserGetPricingResult
	mustNotNil(t, result)
	mustLen(t, result.ProductTypes, 1)
	assert.Equal(t, "domains", result.ProductTypes[0].Name)
	mustLen(t, result.ProductTypes[0].ProductCategories, 2)
	register := result.ProductTypes[0].ProductCategories[0]
	assert.Equal(t, "register", register.Name)
	mustLen(t, register.Products, 3)
	assert.Equal(t, "com", register.Products[0].Name)
	mustLen(t, register.Products[0].Prices, 2)

	// Money is preserved exactly as the server string (never float64).
	comYear1 := register.Products[0].Prices[0]
	assert.Equal(t, 1, comYear1.Duration)
	assert.Equal(t, "YEAR", comYear1.DurationType)
	assert.Equal(t, Amount("8.88"), comYear1.Price)
	assert.Equal(t, Amount("10.87"), comYear1.RegularPrice)
	assert.Equal(t, Amount("9.99"), comYear1.YourPrice)
}

func TestUsersService_GetPricing_PriceForPrecedence(t *testing.T) {
	t.Parallel()

	client := usersMockClient(t, pricingDomainResponse, nil)
	resp, err := client.Users.GetPricingWithContext(context.Background(), &UsersGetPricingArgs{ProductType: String("DOMAIN")})
	mustNoError(t, err)
	result := resp.UserGetPricingResult
	mustNotNil(t, result)

	tests := []struct {
		name      string
		action    string
		product   string
		years     int
		wantFound bool
		wantEff   Amount // expected EffectivePrice
	}{
		{"promo wins (Price set)", "REGISTER", "com", 1, true, "8.88"},
		{"user price when Price zero", "REGISTER", "net", 1, true, "10.50"},
		{"regular when Price empty and user zero", "REGISTER", "org", 1, true, "9.18"},
		{"multi-year tier", "REGISTER", "com", 2, true, "17.76"},
		{"different action", "RENEW", "com", 1, true, "11.00"},
		{"case-insensitive action and product", "register", "COM", 1, true, "8.88"},
		{"unknown product", "REGISTER", "xyz", 1, false, ""},
		{"unknown action", "TRANSFER", "com", 1, false, ""},
		{"unknown duration", "REGISTER", "com", 5, false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			price, ok := result.PriceFor(tc.action, tc.product, tc.years)
			assert.Equal(t, tc.wantFound, ok)
			if tc.wantFound {
				assert.Equal(t, tc.wantEff, price.EffectivePrice())
			}
		})
	}
}

func TestUsersService_GetPricing_SSL(t *testing.T) {
	t.Parallel()

	client := usersMockClient(t, pricingSSLResponse, nil)
	resp, err := client.Users.GetPricingWithContext(context.Background(), &UsersGetPricingArgs{ProductType: String("SSLCERTIFICATE")})
	mustNoError(t, err)

	result := resp.UserGetPricingResult
	mustNotNil(t, result)
	mustLen(t, result.ProductTypes, 1)
	assert.Equal(t, "ssl", result.ProductTypes[0].Name)

	price, ok := result.PriceFor("purchase", "positivessl", 2)
	mustTrue(t, ok)
	assert.Equal(t, Amount("16.00"), price.EffectivePrice())
}

func TestUsersService_GetPricing_Whoisguard(t *testing.T) {
	t.Parallel()

	client := usersMockClient(t, pricingWhoisguardResponse, nil)
	resp, err := client.Users.GetPricingWithContext(context.Background(), &UsersGetPricingArgs{ProductType: String("WHOISGUARD")})
	mustNoError(t, err)

	result := resp.UserGetPricingResult
	mustNotNil(t, result)
	mustLen(t, result.ProductTypes, 1)
	assert.Equal(t, "whoisguard", result.ProductTypes[0].Name)

	price, ok := result.PriceFor("renew", "whoisguard", 1)
	mustTrue(t, ok)
	assert.Equal(t, Amount("2.88"), price.EffectivePrice())
}

func TestUsersService_GetPricing_Validation(t *testing.T) {
	t.Parallel()

	t.Run("nil args no http", func(t *testing.T) {
		t.Parallel()
		var called int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			atomic.AddInt32(&called, 1)
		}))
		defer server.Close()

		client := setupClient(nil)
		client.BaseURL = server.URL

		_, err := client.Users.GetPricingWithContext(context.Background(), nil)
		assertInvalidArguments(t, err, "ProductType")
		assert.Equal(t, int32(0), atomic.LoadInt32(&called))
	})

	t.Run("empty ProductType", func(t *testing.T) {
		t.Parallel()
		client := setupClient(nil)
		_, err := client.Users.GetPricingWithContext(context.Background(), &UsersGetPricingArgs{ProductType: String("")})
		assertInvalidArguments(t, err, "ProductType")
	})

	t.Run("optional params omitted when unset", func(t *testing.T) {
		t.Parallel()
		var sent url.Values
		client := usersMockClient(t, pricingDomainResponse, &sent)
		_, err := client.Users.GetPricingWithContext(context.Background(), &UsersGetPricingArgs{ProductType: String("DOMAIN")})
		mustNoError(t, err)
		assert.Equal(t, "DOMAIN", sent.Get("ProductType"))
		_, hasCategory := sent["ProductCategory"]
		assert.False(t, hasCategory)
		_, hasAction := sent["ActionName"]
		assert.False(t, hasAction)
	})
}

// TestPrice_EffectivePrice unit-tests the precedence directly on Price values,
// independent of parsing, to pin the promo > user > regular fallback.
func TestPrice_EffectivePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    Price
		want Amount
	}{
		{"promo", Price{Price: "8.88", YourPrice: "9.99", RegularPrice: "10.87"}, "8.88"},
		{"user when Price zero string", Price{Price: "0.00", YourPrice: "9.99", RegularPrice: "10.87"}, "9.99"},
		{"user when Price empty", Price{Price: "", YourPrice: "9.99", RegularPrice: "10.87"}, "9.99"},
		{"regular when only regular set", Price{Price: "0.0", YourPrice: "", RegularPrice: "10.87"}, "10.87"},
		{"regular fallthrough all zero", Price{Price: "0.00", YourPrice: "0.00", RegularPrice: "0.00"}, "0.00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.p.EffectivePrice())
		})
	}
}
