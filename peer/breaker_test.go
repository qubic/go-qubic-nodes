package peer

import "testing"

// record n failures (success=false) or successes.
func feed(b *circuitBreaker, results ...bool) {
	for _, r := range results {
		b.record(r)
	}
}

func TestBreakerStaysClosedUntilWindowFills(t *testing.T) {
	b := newCircuitBreaker(4, 0.5, 3)
	// Three failures in a window of four: not full yet, must not trip.
	feed(b, false, false, false)
	if !b.healthy() {
		t.Fatalf("breaker tripped before the window filled")
	}
}

func TestBreakerTripsAtThreshold(t *testing.T) {
	b := newCircuitBreaker(4, 0.5, 3)
	// Window of four, 50% threshold: two failures + two successes trips.
	feed(b, false, true, false, true)
	if b.healthy() {
		t.Fatalf("breaker did not trip at the error threshold")
	}
}

func TestBreakerDoesNotTripBelowThreshold(t *testing.T) {
	b := newCircuitBreaker(4, 0.75, 3)
	// 50% failures, threshold 75%: must stay closed.
	feed(b, false, true, false, true)
	if !b.healthy() {
		t.Fatalf("breaker tripped below the error threshold")
	}
}

func TestSlidingWindowForgetsOldFailures(t *testing.T) {
	b := newCircuitBreaker(4, 0.75, 3)
	// Four failures fill the window (100%) — would trip at 75%... so use a window
	// that then ages out: first fill with failures, then push successes past them.
	feed(b, false, false, false, false) // 100% -> trips
	if b.healthy() {
		t.Fatalf("expected trip at 100%% failures")
	}
	// Now recover and prove the window no longer holds the old failures.
	feed(b, true) // open -> probe succeeds -> reset to closed, window cleared
	if !b.healthy() {
		t.Fatalf("successful probe did not reset the breaker")
	}
	feed(b, false, true, true, true) // 25% failures over a full window, below 75%
	if !b.healthy() {
		t.Fatalf("stale failures were not forgotten by the sliding window")
	}
}

func TestOpenBreakerRecoversOnSuccessfulProbe(t *testing.T) {
	b := newCircuitBreaker(2, 0.5, 3)
	feed(b, false, false) // trip
	if b.healthy() {
		t.Fatal("expected open")
	}
	feed(b, true) // recovery probe succeeds
	if !b.healthy() {
		t.Fatal("expected closed after successful probe")
	}
	if b.shouldRemove() {
		t.Fatal("recovered breaker must not be removable")
	}
}

func TestOpenBreakerBecomesRemovableAfterFailedProbes(t *testing.T) {
	b := newCircuitBreaker(2, 0.5, 3)
	feed(b, false, false) // trip (open, failedProbes=0)
	for i := 0; i < 3; i++ {
		if b.shouldRemove() {
			t.Fatalf("became removable too early, after %d failed probes", i)
		}
		feed(b, false) // failed recovery probe
	}
	if !b.shouldRemove() {
		t.Fatal("expected removable after maxFailedProbes failed probes")
	}
	// A late successful probe still rescues it.
	feed(b, true)
	if b.shouldRemove() || !b.healthy() {
		t.Fatal("a successful probe must clear the removable state")
	}
}

func TestZeroThresholdNeverTrips(t *testing.T) {
	b := newCircuitBreaker(2, 0, 3)
	feed(b, false, false, false, false)
	if !b.healthy() {
		t.Fatal("non-positive threshold must never trip")
	}
}
