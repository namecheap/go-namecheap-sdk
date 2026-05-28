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
}
