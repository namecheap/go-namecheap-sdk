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
	t.Run("invalid_date_string_returns_error", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("not-a-date"))
		assert.Error(t, err)
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

	t.Run("iso_timestamp_returns_error", func(t *testing.T) {
		t.Parallel()
		dt := &DateTime{}
		err := dt.UnmarshalText([]byte("0001-01-01T00:00:00"))
		assert.Error(t, err)
	})
}
