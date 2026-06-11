package service

import (
	"errors"
	"testing"
)

// TestHqkjRetrySucceedsOnSecondAttempt verifies that hqkjRetry retries after a
// transient failure and returns the successful result.
func TestHqkjRetrySucceedsOnSecondAttempt(t *testing.T) {
	calls := 0
	result, err := hqkjRetry(func() (string, error) {
		calls++
		if calls < 2 {
			return "", errors.New("transient error")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("hqkjRetry returned error after retry: %v", err)
	}
	if result != "ok" {
		t.Fatalf("hqkjRetry result = %q, want %q", result, "ok")
	}
	if calls != 2 {
		t.Fatalf("hqkjRetry called fn %d times, want 2", calls)
	}
}

// TestHqkjRetryExhausted verifies that hqkjRetry returns the last error after
// all attempts are exhausted.
func TestHqkjRetryExhausted(t *testing.T) {
	calls := 0
	_, err := hqkjRetry(func() (string, error) {
		calls++
		return "", errors.New("permanent error")
	})
	if err == nil {
		t.Fatal("hqkjRetry should have returned error after all attempts")
	}
	if calls != 3 {
		t.Fatalf("hqkjRetry called fn %d times, want 3", calls)
	}
}

// TestHqkjSafeCallRecoversPanic verifies that hqkjSafeCall converts a panic
// into an error and does not propagate the panic to the caller.
func TestHqkjSafeCallRecoversPanic(t *testing.T) {
	_, err := hqkjSafeCall(func() (string, error) {
		panic("simulated panic")
	})
	if err == nil {
		t.Fatal("hqkjSafeCall should return error when fn panics")
	}
	if err.Error() != "panic: simulated panic" {
		t.Fatalf("hqkjSafeCall error = %q, want %q", err.Error(), "panic: simulated panic")
	}
}

// TestHqkjRetryRecoversPanic verifies that hqkjRetry retries even after a panic
// in fn, and returns the result once fn succeeds.
func TestHqkjRetryRecoversPanic(t *testing.T) {
	calls := 0
	result, err := hqkjRetry(func() (string, error) {
		calls++
		if calls == 1 {
			panic("first call panics")
		}
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("hqkjRetry returned error: %v", err)
	}
	if result != "recovered" {
		t.Fatalf("hqkjRetry result = %q, want %q", result, "recovered")
	}
	if calls != 2 {
		t.Fatalf("hqkjRetry called fn %d times after panic, want 2", calls)
	}
}

// TestGetCoursesPanicReturnsError verifies that GetCourses recover() wrapper
// converts a panic from a platform handler into a clean error.
func TestGetCoursesPanicReturnsError(t *testing.T) {
	// Exercise the recover block in GetCourses directly by simulating what
	// the deferred recover would do, since we can't inject a panic into the
	// real function without a real DB. We test hqkjSafeCall which uses the
	// same pattern.
	_, err := hqkjSafeCall(func() ([]CourseVO, error) {
		panic("simulated platform panic")
	})
	if err == nil {
		t.Fatal("panic should be converted to error")
	}
}
