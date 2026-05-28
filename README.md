# Go Namecheap SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/namecheap/go-namecheap-sdk.svg)](https://pkg.go.dev/github.com/namecheap/go-namecheap-sdk/v2)

- [Namecheap API Documentation](https://www.namecheap.com/support/api/intro/)
- [Sandbox](https://www.namecheap.com/support/knowledgebase/article.aspx/763/63/what-is-sandbox/)

### Getting

```sh
$ go get github.com/namecheap/go-namecheap-sdk/v2
```

### Usage

```go
import "github.com/namecheap/go-namecheap-sdk/v2/namecheap"

client := namecheap.NewClient(&namecheap.ClientOptions{
    UserName:   "UserName",
    ApiUser:    "ApiUser",
    ApiKey:     "ApiKey",
    ClientIp:   "10.10.10.10",
    UseSandbox: false,
})

setHostsResp, err := client.DomainsDNS.SetHosts(&namecheap.DomainsDNSSetHostsArgs{
    Domain: namecheap.String("domain.com"),
    Records: &[]namecheap.DomainsDNSHostRecord{
        {
            HostName:   namecheap.String("blog"),
            RecordType: namecheap.String("A"),
            Address:    namecheap.String("11.12.13.14"),
        },
    },
})

// ...

response, err := client.DomainsDNS.GetHosts("domain.com")

// ...
```

### Sandbox

Before you start using our API, we advise you to try it in our [Sandbox](https://www.sandbox.namecheap.com/) environment. The sandbox environment was created
explicitly for testing purposes. All purchases processed through the sandbox API are simulated.

To start testing API in Sandbox, you will need to sign up for an account here (this account will not be associated with
the one you have at http://www.namecheap.com).

### Contributing

To contribute, please read our [contributing](CONTRIBUTING.md) docs.
