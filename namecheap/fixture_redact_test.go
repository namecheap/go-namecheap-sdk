package namecheap_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fixtureRedaction describes how captureFixture scrubs personal data from a
// response before it is committed. Inside each Container element, the text of
// every element named in Placeholders is replaced with its placeholder, and
// only the elements in Keep and the attributes in Attrs may stay as captured.
// Anything else there (an unknown element, another attribute, nested markup,
// CDATA, a comment, a placeholder element whose text was not replaced) fails
// the capture, so a change in the response cannot put real data into a fixture.
// A response without the Container (an error envelope, a renamed result) fails
// too.
//
// Outside the Container, where the whitelist does not reach, a non-empty
// Forbidden value (one the request sent, such as a profile field) fails the
// capture when it appears as a whole word in a decoded text, comment or
// attribute value, so an escaped echo (A&amp;B for A&B) is caught. The
// envelope's own metadata is not searched: a short value such as PhoneExt "15"
// would otherwise match ExecutionTime "0.15" and fail every capture.
type fixtureRedaction struct {
	Container    string
	Keep         []string
	Attrs        []string
	Placeholders map[string]string
	Forbidden    []string
}

// envelopeElements carry only attributes the API sets itself, and
// envelopeText elements only server metadata; neither can echo a request value.
var (
	envelopeElements = []string{"ApiResponse", "CommandResponse"}
	envelopeText     = []string{"RequestedCommand", "Server", "GMTTimeDifference", "ExecutionTime"}
)

func (r fixtureRedaction) apply(body []byte) ([]byte, error) {
	for name, placeholder := range r.Placeholders {
		plain := regexp.MustCompile(`<` + name + `>[^<]*</` + name + `>`)
		body = plain.ReplaceAllLiteral(body, []byte("<"+name+">"+placeholder+"</"+name+">"))
	}

	dec := xml.NewDecoder(bytes.NewReader(body))
	depth := 0 // 1 inside the container, 2 inside one of its children
	current := ""
	seen := false
	var open []string // elements open outside the container
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if !seen {
				return nil, fmt.Errorf("no <%s> in the response", r.Container)
			}
			return body, nil
		}
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
		if depth == 0 && r.echoes(tok, open) {
			return nil, errors.New("the response echoes a forbidden value")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if t.Name.Local == r.Container {
					if err := r.checkAttrs(t); err != nil {
						return nil, err
					}
					depth = 1
					seen = true
				} else {
					open = append(open, t.Name.Local)
				}
				continue
			}
			_, placeholder := r.Placeholders[t.Name.Local]
			if depth > 1 || (!placeholder && !slices.Contains(r.Keep, t.Name.Local)) {
				return nil, fmt.Errorf("unexpected <%s> in <%s>", t.Name.Local, r.Container)
			}
			if err := r.checkAttrs(t); err != nil {
				return nil, err
			}
			current = t.Name.Local
			depth = 2
		case xml.EndElement:
			if depth > 0 {
				depth--
				current = ""
			} else if len(open) > 0 {
				open = open[:len(open)-1]
			}
		case xml.CharData:
			if placeholder, ok := r.Placeholders[current]; ok && depth == 2 && string(t) != placeholder {
				return nil, fmt.Errorf("<%s> in <%s> is not redacted", current, r.Container)
			}
		case xml.Comment:
			if depth > 0 {
				return nil, fmt.Errorf("unexpected comment in <%s>", r.Container)
			}
		}
	}
}

// echoes reports whether a decoded text, comment or attribute value of tok, a
// token outside the container whose open elements are open, holds a Forbidden
// value as a whole word.
func (r fixtureRedaction) echoes(tok xml.Token, open []string) bool {
	var texts []string
	switch t := tok.(type) {
	case xml.StartElement:
		if t.Name.Local == r.Container || slices.Contains(envelopeElements, t.Name.Local) {
			return false
		}
		for _, a := range t.Attr {
			texts = append(texts, a.Value)
		}
	case xml.CharData:
		if len(open) > 0 && slices.Contains(envelopeText, open[len(open)-1]) {
			return false
		}
		texts = append(texts, string(t))
	case xml.Comment:
		texts = append(texts, string(t))
	}
	for _, v := range r.Forbidden {
		if v == "" {
			continue
		}
		word := regexp.MustCompile(`(^|[^\pL\pN])` + regexp.QuoteMeta(v) + `($|[^\pL\pN])`)
		for _, s := range texts {
			if word.MatchString(s) {
				return true
			}
		}
	}
	return false
}

func (r fixtureRedaction) checkAttrs(t xml.StartElement) error {
	for _, a := range t.Attr {
		if !slices.Contains(r.Attrs, a.Name.Local) {
			return fmt.Errorf("unexpected attribute %s on <%s> in <%s>", a.Name.Local, t.Name.Local, r.Container)
		}
	}
	return nil
}

func TestFixtureRedaction(t *testing.T) {
	info := fixtureRedaction{
		Container:    "Result",
		Keep:         []string{"AddressId"},
		Placeholders: map[string]string{"Phone": "+1.5555555555", "StateProvince": "AZ"},
	}
	list := fixtureRedaction{Container: "Result", Keep: []string{"List"}, Attrs: []string{"AddressId", "IsDefault"}}

	t.Run("listed elements get the placeholder", func(t *testing.T) {
		in := `<R><Server>x</Server><Result><AddressId>0</AddressId><Phone>+380.000</Phone><StateProvince>Kyiv</StateProvince><Phone /></Result></R>`
		got, err := info.apply([]byte(in))
		assert.NoError(t, err)
		assert.Equal(t, `<R><Server>x</Server><Result><AddressId>0</AddressId><Phone>+1.5555555555</Phone><StateProvince>AZ</StateProvince><Phone /></Result></R>`, string(got))
	})

	t.Run("listed attributes stay", func(t *testing.T) {
		in := `<R><Result><List AddressId="0" IsDefault="true" /></Result></R>`
		got, err := list.apply([]byte(in))
		assert.NoError(t, err)
		assert.Equal(t, in, string(got))
	})

	// The shape of the captured users.update response, for an account whose
	// PhoneExt is 15 or 5: neither the whitelisted UserId nor the envelope
	// metadata is an echo.
	t.Run("short forbidden value in envelope metadata stays", func(t *testing.T) {
		update := fixtureRedaction{Container: "Result", Attrs: []string{"UserId"}, Forbidden: []string{"15", "5"}}
		in := `<ApiResponse Status="OK"><Errors /><RequestedCommand>namecheap.users.update</RequestedCommand>` +
			`<CommandResponse Type="namecheap.users.update"><Result UserId="249715" /></CommandResponse>` +
			`<Server>Server -151154064</Server><GMTTimeDifference>--5:00</GMTTimeDifference><ExecutionTime>0.15</ExecutionTime></ApiResponse>`
		got, err := update.apply([]byte(in))
		assert.NoError(t, err)
		assert.Equal(t, in, string(got))
	})

	for _, tt := range []struct {
		name string
		r    fixtureRedaction
		in   string
	}{
		{"unlisted element", info, `<R><Result><Mobile>+380.000</Mobile></Result></R>`},
		{"similar element name", info, `<R><Result><StateProvinceChoice>P</StateProvinceChoice></Result></R>`},
		{"attribute", info, `<R><Result><Phone type="mobile">+380.000</Phone></Result></R>`},
		{"attribute with slash", info, `<R><Result><Phone note="a/b">+380.000</Phone></Result></R>`},
		{"self-closing with attribute", info, `<R><Result><Phone value="+380.000" /></Result></R>`},
		{"attribute on a kept element", info, `<R><Result><AddressId owner="x">0</AddressId></Result></R>`},
		{"attribute on the container", info, `<R><Result email="a@b.c"><AddressId>0</AddressId></Result></R>`},
		{"CDATA", info, `<R><Result><Phone><![CDATA[+380.000]]></Phone></Result></R>`},
		{"nested markup", info, `<R><Result><Phone><Number>+380.000</Number></Phone></Result></R>`},
		{"comment", info, `<R><Result><!-- +380.000 --></Result></R>`},
		{"unlisted list attribute", list, `<R><Result><List AddressId="0" EmailAddress="a@b.c" /></Result></R>`},
		{"forbidden value outside the container", fixtureRedaction{Container: "Result", Forbidden: []string{"a@b.c"}},
			`<R><Warnings><Warning>a@b.c</Warning></Warnings><Result /></R>`},
		{"escaped forbidden value", fixtureRedaction{Container: "Result", Forbidden: []string{"A&B"}},
			`<R><Warnings><Warning>A&amp;B is too long</Warning></Warnings><Result /></R>`},
		{"forbidden value in an attribute outside the container", fixtureRedaction{Container: "Result", Forbidden: []string{"a@b.c"}},
			`<R><Error Field="a@b.c" /><Result /></R>`},
		{"missing container", info, `<R Status="ERROR"><Errors><Error Number="2011166">Invalid value</Error></Errors></R>`},
	} {
		t.Run(tt.name+" fails", func(t *testing.T) {
			got, err := tt.r.apply([]byte(tt.in))
			if err == nil {
				t.Errorf("apply(%q) = %q, want an error", tt.in, got)
			}
		})
	}
}
