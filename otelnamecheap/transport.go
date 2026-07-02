// Package otelnamecheap provides OpenTelemetry tracing for the Namecheap Go SDK
// as an opt-in, separately versioned module, so the core SDK stays free of any
// OpenTelemetry dependency.
//
// Wire it in through the SDK's Transport option, which replaces the RoundTripper
// on the client's HTTP client:
//
//	import (
//	    "github.com/namecheap/go-namecheap-sdk/v2/namecheap"
//	    "github.com/namecheap/go-namecheap-sdk/otelnamecheap"
//	)
//
//	client := namecheap.NewClient(&namecheap.ClientOptions{
//	    UserName: "user", ApiUser: "user", ApiKey: apiKey, ClientIp: ip,
//	    Transport: otelnamecheap.NewTransport(nil), // wrap the default transport
//	})
//
// Every API call then produces one client span per HTTP attempt (the SDK builds
// a fresh request per retry), carrying the Namecheap command and HTTP status as
// attributes and an error status for non-2xx responses or transport failures.
//
// It never records secret parameters: it reads the form body only to extract the
// command name and puts nothing else from the body on the span.
package otelnamecheap

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName is the tracer name reported for spans created by this
// package.
const instrumentationName = "github.com/namecheap/go-namecheap-sdk/otelnamecheap"

// Attribute keys set on every span.
const (
	attrCommand    = attribute.Key("namecheap.command")
	attrHTTPStatus = attribute.Key("http.status_code")
)

// config holds the resolved options for NewTransport.
type config struct {
	tracerProvider trace.TracerProvider
}

// Option customizes the Transport returned by NewTransport.
type Option interface {
	apply(*config)
}

type optionFunc func(*config)

func (f optionFunc) apply(c *config) { f(c) }

// WithTracerProvider sets the trace.TracerProvider used to create spans. When it
// is not supplied, the global provider from otel.GetTracerProvider is used.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return optionFunc(func(c *config) {
		if tp != nil {
			c.tracerProvider = tp
		}
	})
}

// Transport is an http.RoundTripper that starts one OpenTelemetry span per
// request and delegates to a base RoundTripper. Use it via the SDK's
// ClientOptions.Transport.
type Transport struct {
	base   http.RoundTripper
	tracer trace.Tracer
}

// NewTransport wraps base with OpenTelemetry tracing and returns an
// http.RoundTripper suitable for namecheap.ClientOptions.Transport. When base is
// nil, http.DefaultTransport is used.
func NewTransport(base http.RoundTripper, opts ...Option) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	cfg := config{tracerProvider: otel.GetTracerProvider()}
	for _, o := range opts {
		o.apply(&cfg)
	}
	return &Transport{
		base:   base,
		tracer: cfg.tracerProvider.Tracer(instrumentationName),
	}
}

// RoundTrip implements http.RoundTripper. It starts a client span named after
// the Namecheap command, records the HTTP status and sets an error status on the
// span for transport failures or non-2xx responses.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	command := commandFromRequest(req)

	spanName := "namecheap.api"
	if command != "" {
		spanName = command
	}

	ctx, span := t.tracer.Start(req.Context(), spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if command != "" {
		span.SetAttributes(attrCommand.String(command))
	}

	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}

	span.SetAttributes(attrHTTPStatus.Int(resp.StatusCode))
	if resp.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, "HTTP "+strconv.Itoa(resp.StatusCode))
	}
	return resp, nil
}

// commandFromRequest extracts the Namecheap "Command" form field from the
// request body without consuming it: it reads the body, restores it for the base
// RoundTripper, and parses only the command name. It returns "" when the body is
// absent, not form-encoded, or unparsable. It never reads any secret field.
func commandFromRequest(req *http.Request) string {
	if req.Body == nil {
		return ""
	}
	if req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		return ""
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	// Restore the body so the underlying RoundTripper can still send it.
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	if err != nil {
		return ""
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return ""
	}
	return values.Get("Command")
}
