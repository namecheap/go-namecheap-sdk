package syncretry

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testRetryDelays = []int{1, 2, 3}

func TestNewSyncRetry(t *testing.T) {
	t.Parallel()
	t.Run("instance", func(t *testing.T) {
		t.Parallel()
		sr := NewSyncRetry(&Options{testRetryDelays})
		assert.NotNil(t, sr)
	})
}

func TestSyncRetry_Do(t *testing.T) {
	t.Parallel()
	t.Run("one_func_success", func(t *testing.T) {
		t.Parallel()
		sr := NewSyncRetry(&Options{testRetryDelays})
		done := false

		err := sr.Do(func() error {
			done = true
			return nil
		})

		assert.Nil(t, err)
		assert.True(t, done)
	})

	t.Run("two_funcs_sync_success", func(t *testing.T) {
		t.Parallel()
		sr := NewSyncRetry(&Options{testRetryDelays})
		done := 0

		err1 := sr.Do(func() error {
			done++
			return nil
		})

		err2 := sr.Do(func() error {
			done++
			return nil
		})

		assert.Nil(t, err1)
		assert.Nil(t, err2)
		assert.Equal(t, 2, done)
	})

	t.Run("should_forward_error", func(t *testing.T) {
		t.Parallel()
		testError := errors.New("test error")
		sr := NewSyncRetry(&Options{testRetryDelays})

		err := sr.Do(func() error {
			return testError
		})

		assert.NotNil(t, err)
		assert.ErrorIs(t, testError, err)
	})

	t.Run("two_funcs_parallel_success", func(t *testing.T) {
		t.Parallel()
		sr := NewSyncRetry(&Options{testRetryDelays})
		done := int32(0)

		firstDone := make(chan error)
		secondDone := make(chan error)

		go func() {
			err := sr.Do(func() error {
				atomic.AddInt32(&done, 1)
				time.Sleep(time.Millisecond * time.Duration(200))
				return nil
			})

			firstDone <- err
		}()

		go func() {
			err := sr.Do(func() error {
				atomic.AddInt32(&done, 1)
				time.Sleep(time.Millisecond * time.Duration(200))
				return nil
			})

			secondDone <- err
		}()

		err1 := <-firstDone
		err2 := <-secondDone

		assert.Equal(t, int32(2), done)
		assert.Nil(t, err1)
		assert.Nil(t, err2)
	})

	t.Run("one_func_retry_last_success", func(t *testing.T) {
		t.Parallel()
		delays := []int{1, 1, 1}
		sr := NewSyncRetry(&Options{delays})
		count := 0

		err := sr.Do(func() error {
			if count == len(testRetryDelays) {
				return nil
			}
			count++
			return ErrRetry
		})

		assert.Nil(t, err)
		assert.Equal(t, len(delays), count)
	})

	t.Run("one_func_exceed_error", func(t *testing.T) {
		t.Parallel()
		delays := []int{1, 1}
		sr := NewSyncRetry(&Options{delays})
		count := 0

		err := sr.Do(func() error {
			count++
			return ErrRetry
		})

		assert.ErrorIs(t, ErrRetryAttempts, err)
	})

	t.Run("two_func_retry_success", func(t *testing.T) {
		t.Parallel()
		delays := []int{1, 1, 1}
		sr := NewSyncRetry(&Options{delays})

		firstFuncCalls := int32(0)
		secondFuncCalls := int32(0)

		firstDone := make(chan error)
		secondDone := make(chan error)

		go func() {
			count := 0
			err := sr.Do(func() error {
				count++
				atomic.AddInt32(&firstFuncCalls, 1)
				if count != 2 {
					return ErrRetry
				}
				return nil
			})

			firstDone <- err
		}()

		go func() {
			count := 0
			err := sr.Do(func() error {
				count++
				atomic.AddInt32(&secondFuncCalls, 1)
				if count != 2 {
					return ErrRetry
				}
				return nil
			})

			secondDone <- err
		}()

		err1 := <-firstDone
		err2 := <-secondDone

		assert.Equal(t, int32(2), firstFuncCalls)
		assert.Equal(t, int32(2), secondFuncCalls)
		assert.Nil(t, err1)
		assert.Nil(t, err2)
	})

	t.Run("parallel_funcs_exceeded_error", func(t *testing.T) {
		t.Parallel()
		delays := []int{1, 1}
		sr := NewSyncRetry(&Options{delays})

		firstFuncCalls := int32(0)
		secondFuncCalls := int32(0)

		firstDone := make(chan error)
		secondDone := make(chan error)

		go func() {
			count := 0
			err := sr.Do(func() error {
				count++
				atomic.AddInt32(&firstFuncCalls, 1)
				return ErrRetry
			})

			firstDone <- err
		}()

		go func() {
			count := 0
			err := sr.Do(func() error {
				count++
				atomic.AddInt32(&secondFuncCalls, 1)
				return ErrRetry
			})

			secondDone <- err
		}()

		err1 := <-firstDone
		err2 := <-secondDone

		assert.Equal(t, int32(3), firstFuncCalls)
		assert.Equal(t, int32(3), secondFuncCalls)
		assert.ErrorIs(t, ErrRetryAttempts, err1)
		assert.ErrorIs(t, ErrRetryAttempts, err2)
	})

	t.Run("non_retry_error_during_retry_loop", func(t *testing.T) {
		t.Parallel()
		delays := []int{1, 1, 1}
		sr := NewSyncRetry(&Options{delays})
		nonRetryErr := errors.New("non-retriable error")
		count := 0

		err := sr.Do(func() error {
			count++
			if count == 1 {
				return ErrRetry
			}
			return nonRetryErr
		})

		assert.ErrorIs(t, err, nonRetryErr)
		assert.Equal(t, 2, count)
	})
}
