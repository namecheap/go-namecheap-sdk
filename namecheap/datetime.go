package namecheap

import (
	"strings"
	"time"
)

// DateTime represents a time that can be unmarshalled from an XML
type DateTime struct {
	time.Time
}

func (dt DateTime) String() string {
	return dt.Time.String()
}

// UnmarshalText parses an API date in MM/DD/YYYY or M/D/YYYY form (the API is
// inconsistent about zero-padding across endpoints), ignoring surrounding
// whitespace. Empty or whitespace-only input yields the zero time without
// error: the API omits or empties date elements for domains that lack them,
// and failing there would abort decoding of the entire response.
func (dt *DateTime) UnmarshalText(text []byte) (err error) {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		dt.Time = time.Time{}
		return nil
	}

	dt.Time, err = time.Parse("1/2/2006", trimmed)
	return err
}

// Equal reports whether dt and u are equal based on time.Equal
func (dt DateTime) Equal(u DateTime) bool {
	return dt.Time.Equal(u.Time)
}
