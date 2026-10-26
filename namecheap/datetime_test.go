package namecheap

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDateTimeString(t *testing.T) {
	t.Parallel()
	t.Run("returns_string_representation", func(t *testing.T) {
		t.Parallel()
		dt := DateTime{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)}
		result := dt.String()
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "2024")
	})
}

func TestDateTimeEqual(t *testing.T) {
	t.Parallel()
	t.Run("equal_datetimes_return_true", func(t *testing.T) {
		t.Parallel()
		dt1 := DateTime{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)}
		dt2 := DateTime{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)}
		assert.True(t, dt1.Equal(dt2))
	})

	t.Run("different_datetimes_return_false", func(t *testing.T) {
		t.Parallel()
		dt1 := DateTime{Time: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)}
		dt2 := DateTime{Time: time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC)}
		assert.False(t, dt1.Equal(dt2))
	})
}

func TestDateTimeUnmarshalText(t *testing.T) {
	t.Parallel()
	// A value the parser does not recognise must never abort decoding of the
	// whole response (renew is charge-bearing and never retried): it is kept
	// verbatim in Raw, Time stays zero, and ParseError reports why.
	t.Run("unrecognised_value_is_kept_raw_without_error", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("not-a-date"))
		assert.NoError(t, err)
		assert.True(t, dt.IsZero())
		assert.Equal(t, "not-a-date", dt.Raw)
		assert.Error(t, dt.ParseError())
		assert.Equal(t, "not-a-date", dt.String())
	})

	t.Run("empty_input_yields_zero_time", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte(""))
		assert.NoError(t, err)
		assert.True(t, dt.IsZero())
	})

	t.Run("whitespace_only_input_yields_zero_time", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("\n\t \n"))
		assert.NoError(t, err)
		assert.True(t, dt.IsZero())
	})

	t.Run("padded_date_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("\n\t11/26/2021\n"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 11, 26, 0, 0, 0, 0, time.UTC)}))
	})

	t.Run("plain_date_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("11/26/2021"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 11, 26, 0, 0, 0, 0, time.UTC)}))
	})

	t.Run("non_padded_date_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("2/20/2019"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2019, 2, 20, 0, 0, 0, 0, time.UTC)}))
	})

	// ISO form, as PremiumDnsSubscription CreatedDate/ExpirationDate use it
	// (production getInfo capture 2026-10-23: 0001-01-01T00:00:00 = "never").
	t.Run("iso_timestamp_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("2027-06-02T13:01:07"))
		assert.NoError(t, err)
		assert.NoError(t, dt.ParseError())
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2027, 6, 2, 13, 1, 7, 0, time.UTC)}))
	})

	t.Run("iso_never_sentinel_is_zero_without_error", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("0001-01-01T00:00:00"))
		assert.NoError(t, err)
		assert.NoError(t, dt.ParseError())
		assert.True(t, dt.IsZero())
	})

	// Captured from a real sandbox domains.renew on 2026-10-26:
	// <ExpiredDate>12/14/2034 1:01:07\u202fPM</ExpiredDate>. The hour is not
	// padded and the AM/PM separator is U+202F, a narrow no-break space.
	t.Run("renew_time_with_narrow_no_break_space_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("12/14/2034 1:01:07\u202fPM"))
		assert.NoError(t, err)
		assert.NoError(t, dt.ParseError())
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2034, 12, 14, 13, 1, 7, 0, time.UTC)}))
		assert.Equal(t, "12/14/2034 1:01:07\u202fPM", dt.Raw)
	})

	// The same field as shown on the official renew docs page (ASCII space).
	t.Run("renew_time_with_ascii_space_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("4/30/2021 11:31:13 AM"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 4, 30, 11, 31, 13, 0, time.UTC)}))
	})

	t.Run("time_with_no_break_space_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("4/30/2021 11:31:13\u00a0PM"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 4, 30, 23, 31, 13, 0, time.UTC)}))
	})

	// A host with a 24-hour culture (invariant culture renders this form).
	t.Run("time_24h_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("04/30/2021 23:31:13"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 4, 30, 23, 31, 13, 0, time.UTC)}))
	})

	// Reported by a consumer on go-namecheap-sdk PR #177 (seen before 2025).
	t.Run("two_digit_year_date_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("4/30/21"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 4, 30, 0, 0, 0, 0, time.UTC)}))
	})

	t.Run("two_digit_year_time_parses", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("4/30/21 11:31:13 AM"))
		assert.NoError(t, err)
		assert.True(t, dt.Equal(DateTime{Time: time.Date(2021, 4, 30, 11, 31, 13, 0, time.UTC)}))
	})

	t.Run("parsed_value_keeps_raw_text", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte(" 11/19/2026 "))
		assert.NoError(t, err)
		assert.Equal(t, "11/19/2026", dt.Raw)
		assert.NoError(t, dt.ParseError())
	})
}

// Parsed is the one-call safe accessor: a consumer that reads Time directly
// gets the zero time for an unparsed value, which looks like a valid year-1
// date. Parsed returns ok=false for both an unparsed and an empty value.
func TestDateTimeParsed(t *testing.T) {
	t.Parallel()
	t.Run("parsed_value_returns_time_and_true", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		assert.NoError(t, dt.UnmarshalText([]byte("12/14/2034 1:01:07 PM")))
		got, ok := dt.Parsed()
		assert.True(t, ok)
		assert.Equal(t, time.Date(2034, 12, 14, 13, 1, 7, 0, time.UTC), got)
	})

	t.Run("unrecognised_value_returns_false", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		assert.NoError(t, dt.UnmarshalText([]byte("not-a-date")))
		got, ok := dt.Parsed()
		assert.False(t, ok)
		assert.True(t, got.IsZero())
	})

	t.Run("empty_value_returns_false", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		assert.NoError(t, dt.UnmarshalText([]byte("")))
		_, ok := dt.Parsed()
		assert.False(t, ok)
	})

	t.Run("nil_receiver_returns_false", func(t *testing.T) {
		t.Parallel()
		var dt *DateTime
		_, ok := dt.Parsed()
		assert.False(t, ok)
	})
}
