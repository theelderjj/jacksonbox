package gateway

import (
	"fmt"
	"testing"
	"time"
)

func TestIdempotency_UniqueIDsPass(t *testing.T) {
	t.Log("Scenario: 10 unique message IDs submitted in sequence within the TTL window.")
	t.Log("Expected: Seen returns false for every one — no ID has been observed before.")
	var r Idempotency
	now := time.Now()
	for i := 0; i < 10; i++ {
		if r.Seen(fmt.Sprintf("id-%d", i), now) {
			t.Fatalf("id-%d unexpectedly seen", i)
		}
	}
}

func TestIdempotency_DupeRejected(t *testing.T) {
	t.Log("Scenario: the same message ID `x` is submitted twice at the same timestamp.")
	t.Log("Expected: first Seen returns false (fresh); second Seen returns true (duplicate) — protects against client retries.")
	var r Idempotency
	now := time.Now()
	if r.Seen("x", now) {
		t.Fatal("first Seen should return false")
	}
	if !r.Seen("x", now) {
		t.Fatal("second Seen should return true")
	}
}

func TestIdempotency_TTLExpiry(t *testing.T) {
	t.Logf("Scenario: ID `old` seen at t0, then re-submitted at t0 + (ringTTL + 1s) — past the retention window of %s.", ringTTL)
	t.Log("Expected: the late resubmission is treated as fresh — IDs older than ringTTL are no longer tracked.")
	var r Idempotency
	t0 := time.Now()
	r.Seen("old", t0)
	// jump past the TTL window
	if r.Seen("old", t0.Add(ringTTL+time.Second)) {
		t.Fatal("expired id should be treated as fresh")
	}
}

func TestIdempotency_RingEvicts(t *testing.T) {
	t.Logf("Scenario: push 2×ringSize (%d) unique IDs, then re-check the first one at the same timestamp.", ringSize*2)
	t.Log("Expected: id-0 appears fresh again — the ring buffer has evicted it by wrapping past its slot. Bounded memory, TTL backstop.")
	var r Idempotency
	now := time.Now()
	// Fill past the ring size; old entries should be recycled.
	for i := 0; i < ringSize*2; i++ {
		r.Seen(fmt.Sprintf("id-%d", i), now)
	}
	// Earliest IDs should now be forgotten (have wrapped off).
	if r.Seen("id-0", now) {
		t.Fatal("id-0 should have been evicted")
	}
}
