package namecheap

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The existing test suite depends only on testify/assert (require is not
// vendored). These fail-fast helpers give the require.* semantics the users tests
// need — abort the current test on failure so a following pointer dereference is
// safe — without adding a dependency.

// mustNoError aborts the test when err is non-nil.
func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// mustNotNil aborts the test when v is nil (including a typed nil pointer/slice).
func mustNotNil(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		t.Fatal("expected non-nil value, got nil")
	}
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface, reflect.Chan, reflect.Func:
		if rv.IsNil() {
			t.Fatal("expected non-nil value, got typed nil")
		}
	}
}

// mustLen aborts the test when v (a slice/array/map/string) does not have length n.
func mustLen(t *testing.T, v any, n int) {
	t.Helper()
	if got := reflect.ValueOf(v).Len(); got != n {
		t.Fatalf("expected length %d, got %d", n, got)
	}
}

// mustTrue aborts the test when ok is false.
func mustTrue(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("expected true, got false")
	}
}

// assertInvalidArguments asserts err is an *InvalidArgumentsError whose Fields
// contain every name in wantFields.
func assertInvalidArguments(t *testing.T, err error, wantFields ...string) {
	t.Helper()
	var argErr *InvalidArgumentsError
	if !assert.True(t, errors.As(err, &argErr), "expected *InvalidArgumentsError, got %v", err) {
		return
	}
	for _, f := range wantFields {
		assert.Contains(t, argErr.Fields, f)
	}
}

// assertAPIError asserts err unwraps to an *APIError carrying wantNumber.
func assertAPIError(t *testing.T, err error, wantNumber int) {
	t.Helper()
	var apiErr *APIError
	if assert.True(t, errors.As(err, &apiErr), "expected *APIError, got %v", err) {
		assert.Equal(t, wantNumber, apiErr.Number)
	}
}

// usersMockClient returns a client pointed at a mock server that serves body and
// records the last request's decoded form values into sent (when non-nil).
func usersMockClient(t *testing.T, body string, sent *url.Values) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if sent != nil {
			q, _ := url.ParseQuery(string(raw))
			*sent = q
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	client := setupClient(nil)
	client.BaseURL = server.URL
	return client
}

// validAddressDetails returns a fully-populated address entry that passes
// validation, ready to be tweaked per test.
func validAddressDetails() *UsersAddressDetails {
	return &UsersAddressDetails{
		AddressName:         "Home",
		EmailAddress:        "john@example.com",
		FirstName:           "John",
		LastName:            "Smith",
		JobTitle:            "Dev",
		Organization:        "NameCheap.com",
		Address1:            "8939 S.cross Blvd",
		Address2:            "Suite 600",
		City:                "Phoenix",
		StateProvince:       "AZ",
		StateProvinceChoice: "S",
		Zip:                 "85284",
		Country:             "US",
		Phone:               "+1.6613102107",
		PhoneExt:            "123",
		Fax:                 "+1.6613102108",
	}
}
