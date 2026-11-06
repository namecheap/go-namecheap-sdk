package namecheap_test

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// redactElements replaces the text of every element named in placeholders with
// its placeholder. It fails closed: when an element has a closing tag that the
// replacement did not reach (attributes, CDATA, nested markup), it returns an
// error, so a response shape change cannot put real data into a committed
// fixture. Self-closing elements carry no value and are left alone.
func redactElements(body []byte, placeholders map[string]string) ([]byte, error) {
	for name, placeholder := range placeholders {
		redacted := []byte("<" + name + ">" + placeholder + "</" + name + ">")
		plain := regexp.MustCompile(`<` + name + `>[^<]*</` + name + `>`)
		body = plain.ReplaceAllLiteral(body, redacted)

		closing := regexp.MustCompile(`</` + name + `\s*>`)
		if all, done := len(closing.FindAll(body, -1)), bytes.Count(body, redacted); all != done {
			return nil, fmt.Errorf("%d <%s> elements, %d redacted", all, name, done)
		}
	}
	return body, nil
}

func TestRedactElements(t *testing.T) {
	placeholders := map[string]string{"Phone": "+1.5555555555", "StateProvince": "AZ"}

	t.Run("plain elements get the placeholder", func(t *testing.T) {
		in := `<R><Phone>+380.000</Phone><StateProvince>Kyiv</StateProvince><StateProvinceChoice>P</StateProvinceChoice><Fax /></R>`
		got, err := redactElements([]byte(in), placeholders)
		assert.NoError(t, err)
		assert.Equal(t, `<R><Phone>+1.5555555555</Phone><StateProvince>AZ</StateProvince><StateProvinceChoice>P</StateProvinceChoice><Fax /></R>`, string(got))
	})

	for _, tt := range []struct{ name, in string }{
		{"attribute", `<R><Phone type="mobile">+380.000</Phone></R>`},
		{"attribute with slash", `<R><Phone note="a/b">+380.000</Phone></R>`},
		{"CDATA", `<R><Phone><![CDATA[+380.000]]></Phone></R>`},
		{"nested markup", `<R><Phone><Number>+380.000</Number></Phone></R>`},
	} {
		t.Run(tt.name+" fails closed", func(t *testing.T) {
			got, err := redactElements([]byte(tt.in), placeholders)
			if err == nil {
				t.Errorf("redactElements(%q) = %q, want an error", tt.in, got)
			}
		})
	}
}
