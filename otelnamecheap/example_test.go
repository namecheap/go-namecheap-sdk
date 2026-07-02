package otelnamecheap_test

import (
	"github.com/namecheap/go-namecheap-sdk/v2/namecheap"
	"github.com/namecheap/go-namecheap-sdk/v2/otelnamecheap"
)

// ExampleNewTransport shows how to enable OpenTelemetry tracing for the SDK by
// wiring the tracing RoundTripper through ClientOptions.Transport. Passing nil
// wraps http.DefaultTransport; the global TracerProvider is used unless
// WithTracerProvider is supplied.
func ExampleNewTransport() {
	client := namecheap.NewClient(&namecheap.ClientOptions{
		UserName: "user",
		ApiUser:  "user",
		ApiKey:   "your-api-key",
		ClientIp: "10.0.0.1",
		Transport: otelnamecheap.NewTransport(nil,
			otelnamecheap.WithTracerProvider(nil), // omit to use the global provider
		),
	})
	_ = client
}
