package syncretry

import (
	"context"
	"errors"
	"sync"
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

func TestDoContextCancelsWhileWaitingForSemaphore(t *testing.T) {
	t.Parallel()
	sr := NewSyncRetry(&Options{Delays: []int{30}})

	g1Holding := make(chan struct{})
	var once sync.Once

	// Goroutine 1 enters the retry section and holds the semaphore for the
	// duration of a 30s inter-retry sleep.
	go func() {
		_ = sr.DoContext(context.Background(), func(context.Context) error {
			once.Do(func() { close(g1Holding) })
			return ErrRetry
		})
	}()

	// Wait for goroutine 1's first attempt, then give it a moment to acquire
	// the semaphore and start sleeping while holding it.
	<-g1Holding
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	result := make(chan error, 1)
	start := time.Now()
	go func() {
		// This call returns ErrRetry on its first attempt, then blocks waiting
		// for the semaphore held by goroutine 1 until ctx is cancelled.
		result <- sr.DoContext(ctx, func(context.Context) error {
			return ErrRetry
		})
	}()

	err := <-result
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, elapsed, 5*time.Second, "should return promptly instead of waiting out the 30s delay")
}

func TestDoContextCancelledUpFront(t *testing.T) {
	t.Parallel()
	sr := NewSyncRetry(&Options{Delays: []int{1, 1}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invoking DoContext

	var called atomic.Bool
	err := sr.DoContext(ctx, func(context.Context) error {
		called.Store(true)
		return nil
	})

	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called.Load(), "f must not be called when ctx is already cancelled")
}
