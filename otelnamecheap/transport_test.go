package otelnamecheap

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// stubRT is a base RoundTripper that returns a canned status without touching
// the network, and records the request body it received.
type stubRT struct {
	status  int
	gotBody string
}

func (s *stubRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		s.gotBody = string(body)
		_ = req.Body.Close()
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(strings.NewReader("<ApiResponse/>")),
		Header:     make(http.Header),
	}, nil
}

func newRecorder() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	return tp, sr
}

func formRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://api.namecheap.com/xml.response", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func attrValue(kvs []attribute.KeyValue, key attribute.Key) (attribute.Value, bool) {
	for _, kv := range kvs {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestNewTransportRecordsSpan asserts a successful call produces one span with
// the documented command and http.status_code attributes, and that no secret
// from the request body leaks onto the span.
func TestNewTransportRecordsSpan(t *testing.T) {
	tp, sr := newRecorder()
	base := &stubRT{status: http.StatusOK}
	rt := NewTransport(base, WithTracerProvider(tp))

	const secret = "SUPERSECRET-apikey"
	req := formRequest(t, "Command=namecheap.domains.getInfo&ApiKey="+secret+"&DomainName=example.com")
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// The base transport must still receive the full, unmodified body.
	if !strings.Contains(base.gotBody, "ApiKey="+secret) {
		t.Fatalf("base transport lost the request body: %q", base.gotBody)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	span := spans[0]

	if span.Name() != "namecheap.domains.getInfo" {
		t.Errorf("span name = %q, want the command", span.Name())
	}

	cmd, ok := attrValue(span.Attributes(), attrCommand)
	if !ok || cmd.AsString() != "namecheap.domains.getInfo" {
		t.Errorf("command attribute = %v (present=%v), want namecheap.domains.getInfo", cmd.AsString(), ok)
	}
	status, ok := attrValue(span.Attributes(), attrHTTPStatus)
	if !ok || status.AsInt64() != int64(http.StatusOK) {
		t.Errorf("http.status_code attribute = %d (present=%v), want 200", status.AsInt64(), ok)
	}
	if span.Status().Code != codes.Unset {
		t.Errorf("status code = %v, want Unset for a 2xx response", span.Status().Code)
	}

	// Redaction sanity: the secret must not appear anywhere on the span.
	for _, kv := range span.Attributes() {
		if strings.Contains(kv.Value.Emit(), secret) {
			t.Errorf("secret leaked into span attribute %s", kv.Key)
		}
	}
}

// TestNewTransportErrorStatus asserts a non-2xx response sets an error status on
// the span.
func TestNewTransportErrorStatus(t *testing.T) {
	tp, sr := newRecorder()
	rt := NewTransport(&stubRT{status: http.StatusInternalServerError}, WithTracerProvider(tp))

	req := formRequest(t, "Command=namecheap.domains.getInfo")
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if got := spans[0].Status().Code; got != codes.Error {
		t.Errorf("status code = %v, want Error for a 500 response", got)
	}
	status, _ := attrValue(spans[0].Attributes(), attrHTTPStatus)
	if status.AsInt64() != int64(http.StatusInternalServerError) {
		t.Errorf("http.status_code = %d, want 500", status.AsInt64())
	}
}

// errRT is a base RoundTripper that always fails, to exercise the transport
// error path.
type errRT struct{ err error }

func (e errRT) RoundTrip(*http.Request) (*http.Response, error) { return nil, e.err }

// TestNewTransportTransportError asserts a transport failure records the error
// on the span and propagates it.
func TestNewTransportTransportError(t *testing.T) {
	tp, sr := newRecorder()
	wantErr := errors.New("connection reset")
	rt := NewTransport(errRT{err: wantErr}, WithTracerProvider(tp))

	req := formRequest(t, "Command=namecheap.domains.getInfo")
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("expected an error")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if got := spans[0].Status().Code; got != codes.Error {
		t.Errorf("status code = %v, want Error on transport failure", got)
	}
}
