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
// A non-empty Forbidden value (one the request sent, such as a profile field)
// fails it wherever it appears in the redacted body.
type fixtureRedaction struct {
	Container    string
	Keep         []string
	Attrs        []string
	Placeholders map[string]string
	Forbidden    []string
}

func (r fixtureRedaction) apply(body []byte) ([]byte, error) {
	for name, placeholder := range r.Placeholders {
		plain := regexp.MustCompile(`<` + name + `>[^<]*</` + name + `>`)
		body = plain.ReplaceAllLiteral(body, []byte("<"+name+">"+placeholder+"</"+name+">"))
	}
	for _, v := range r.Forbidden {
		if v != "" && bytes.Contains(body, []byte(v)) {
			return nil, errors.New("the response echoes a forbidden value")
		}
	}

	dec := xml.NewDecoder(bytes.NewReader(body))
	depth := 0 // 1 inside the container, 2 inside one of its children
	current := ""
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return body, nil
		}
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if t.Name.Local == r.Container {
					if err := r.checkAttrs(t); err != nil {
						return nil, err
					}
					depth = 1
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
	} {
		t.Run(tt.name+" fails", func(t *testing.T) {
			got, err := tt.r.apply([]byte(tt.in))
			if err == nil {
				t.Errorf("apply(%q) = %q, want an error", tt.in, got)
			}
		})
	}
}
