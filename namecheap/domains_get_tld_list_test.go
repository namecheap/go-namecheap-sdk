package namecheap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const domainsGetTldListOKResponse = `
	<?xml version="1.0" encoding="utf-8"?>
	<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
		<Errors />
		<Warnings />
		<RequestedCommand>namecheap.domains.getTldList</RequestedCommand>
		<CommandResponse Type="namecheap.domains.getTldList">
			<Tlds>
				<Tld Name="com" NonRealTimeDomain="false" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="1" IsApiRegisterable="true" IsApiRenewable="true" IsApiTransferable="false" IsEppRequired="true">.com domain</Tld>
				<Tld Name="net" NonRealTimeDomain="false" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="1" IsApiRegisterable="true" IsApiRenewable="true" IsApiTransferable="true" IsEppRequired="false">.net domain</Tld>
			</Tlds>
		</CommandResponse>
		<Server>PHX01SBAPIEXT06</Server>
		<GMTTimeDifference>--4:00</GMTTimeDifference>
		<ExecutionTime>0.9</ExecutionTime>
	</ApiResponse>
`

func TestDomainsService_GetTldList(t *testing.T) {
	t.Parallel()

	t.Run("success_parse_and_command", func(t *testing.T) {
		t.Parallel()
		var sentBody url.Values
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			sentBody, _ = url.ParseQuery(string(body))
			_, _ = writer.Write([]byte(domainsGetTldListOKResponse))
		}))
		defer mockServer.Close()

		client := setupClient(nil)
		client.BaseURL = mockServer.URL

		resp, err := client.Domains.GetTldListWithContext(context.Background())
		if err != nil {
			t.Fatal("unexpected error", err)
		}

		assert.Equal(t, "namecheap.domains.getTldList", sentBody.Get("Command"))

		tlds := *resp.Tlds
		assert.Len(t, tlds, 2)

		com := tlds[0]
		assert.Equal(t, "com", *com.Name)
		assert.Equal(t, false, *com.NonRealTimeDomain)
		assert.Equal(t, 1, *com.MinRegisterYears)
		assert.Equal(t, 10, *com.MaxRegisterYears)
		assert.Equal(t, true, *com.IsAPIRegisterable)
		assert.Equal(t, true, *com.IsAPIRenewable)
		assert.Equal(t, false, *com.IsAPITransferable)
		assert.Equal(t, true, *com.IsEppRequired)

		net := tlds[1]
		assert.Equal(t, "net", *net.Name)
		assert.Equal(t, true, *net.IsAPITransferable)
		assert.Equal(t, false, *net.IsEppRequired)
	})

	t.Run("api_error_mapped_to_APIError", func(t *testing.T) {
		t.Parallel()
		mockServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
					<Errors><Error Number="5050900">Unknown exceptions</Error></Errors>
					<CommandResponse/>
				</ApiResponse>`))
		}))
		defer mockServer.Close()

		client := setupNoRetryClient()
		client.BaseURL = mockServer.URL

		_, err := client.Domains.GetTldListWithContext(context.Background())
		var apiErr *APIError
		if assert.True(t, errors.As(err, &apiErr)) {
			assert.Equal(t, 5050900, apiErr.Number)
		}
	})
}
