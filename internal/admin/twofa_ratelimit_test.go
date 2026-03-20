package admin

import (
	"testing"
	"time"
)

func TestRecordFailure_IncrementsCount(t *testing.T) {
	rl := newTwoFARateLimiter(5, 5*time.Minute)

	for i := 0; i < 4; i++ {
		blocked := rl.RecordFailure("user1")
		if blocked {
			t.Fatalf("expected not blocked after %d failures", i+1)
		}
	}

	blocked := rl.RecordFailure("user1")
	if !blocked {
		t.Fatal("expected blocked after 5 failures")
	}
}

func TestIsBlocked_TriggersAtThreshold(t *testing.T) {
	rl := newTwoFARateLimiter(3, 5*time.Minute)

	if rl.IsBlocked("user1") {
		t.Fatal("should not be blocked with no attempts")
	}

	rl.RecordFailure("user1")
	rl.RecordFailure("user1")
	if rl.IsBlocked("user1") {
		t.Fatal("should not be blocked after 2 failures with threshold 3")
	}

	rl.RecordFailure("user1")
	if !rl.IsBlocked("user1") {
		t.Fatal("should be blocked after 3 failures with threshold 3")
	}
}

func TestIsBlocked_IsolatesUsers(t *testing.T) {
	rl := newTwoFARateLimiter(2, 5*time.Minute)

	rl.RecordFailure("user1")
	rl.RecordFailure("user1")

	if !rl.IsBlocked("user1") {
		t.Fatal("user1 should be blocked")
	}
	if rl.IsBlocked("user2") {
		t.Fatal("user2 should not be blocked")
	}
}

func TestClear_ResetsCounter(t *testing.T) {
	rl := newTwoFARateLimiter(3, 5*time.Minute)

	rl.RecordFailure("user1")
	rl.RecordFailure("user1")
	rl.RecordFailure("user1")

	if !rl.IsBlocked("user1") {
		t.Fatal("should be blocked")
	}

	rl.Clear("user1")

	if rl.IsBlocked("user1") {
		t.Fatal("should not be blocked after Clear")
	}
}

func TestCleanup_RemovesStaleEntries(t *testing.T) {
	rl := newTwoFARateLimiter(2, 50*time.Millisecond)

	rl.RecordFailure("user1")
	rl.RecordFailure("user1")

	if !rl.IsBlocked("user1") {
		t.Fatal("should be blocked")
	}

	time.Sleep(60 * time.Millisecond)
	rl.Cleanup()

	if rl.IsBlocked("user1") {
		t.Fatal("should not be blocked after cleanup of expired entry")
	}
}

func TestIsBlocked_ExpiresNaturally(t *testing.T) {
	rl := newTwoFARateLimiter(2, 50*time.Millisecond)

	rl.RecordFailure("user1")
	rl.RecordFailure("user1")

	if !rl.IsBlocked("user1") {
		t.Fatal("should be blocked")
	}

	time.Sleep(60 * time.Millisecond)

	if rl.IsBlocked("user1") {
		t.Fatal("should not be blocked after window expires")
	}
}

func TestRecordFailure_ResetsAfterWindowExpires(t *testing.T) {
	rl := newTwoFARateLimiter(3, 50*time.Millisecond)

	rl.RecordFailure("user1")
	rl.RecordFailure("user1")

	time.Sleep(60 * time.Millisecond)

	// After window expires, count should reset
	blocked := rl.RecordFailure("user1")
	if blocked {
		t.Fatal("should not be blocked - window expired and count should have reset")
	}
}
