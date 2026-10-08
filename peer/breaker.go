package peer

// circuitBreaker tracks recent call outcomes for a single node and decides when
// the node should be timed out and, after repeated failed recovery probes,
// removed from the pool. See issue #19.
//
// The window is CALL based, not time based: it counts a fixed number of the most
// recent calls. A call is whatever the Manager records each update cycle — a
// reachable node that is within the tick-delay threshold is a success, anything
// else (unreachable, i/o timeout, too far behind the network tick) is a failure.
//
// States:
//   - closed: healthy. Outcomes fill a sliding window; once the window is full and
//     the failure rate reaches the threshold, the breaker trips to open.
//   - open:   timed out. The node is excluded from selection but still probed once
//     per cycle; that probe is the recovery attempt. A success resets the breaker
//     to closed; a failure increments the failed-probe counter, and once it
//     reaches maxFailedProbes the node is eligible for removal.
//
// Not safe for concurrent use; the Manager serialises access under its peer mutex.
type circuitBreaker struct {
	windowSize      int
	errorThreshold  float64 // fraction in [0,1]; trip when failureRate >= this and the window is full
	maxFailedProbes int

	outcomes []bool // ring buffer of the last windowSize call results (true = success)
	head     int
	filled   int
	failures int

	open         bool
	failedProbes int
}

func newCircuitBreaker(windowSize int, errorThreshold float64, maxFailedProbes int) *circuitBreaker {
	if windowSize < 1 {
		windowSize = 1
	}
	if errorThreshold <= 0 {
		errorThreshold = 2 // disabled: a failure rate (max 1.0) can never reach it, so it never trips
	}
	return &circuitBreaker{
		windowSize:      windowSize,
		errorThreshold:  errorThreshold,
		maxFailedProbes: maxFailedProbes,
		outcomes:        make([]bool, windowSize),
	}
}

// record feeds one call outcome and advances the state machine.
func (b *circuitBreaker) record(success bool) {
	if b.open {
		// A call while open is the recovery probe.
		if success {
			b.reset()
		} else {
			b.failedProbes++
		}
		return
	}

	b.push(success)
	if b.filled >= b.windowSize && b.failureRate() >= b.errorThreshold {
		b.open = true
		b.failedProbes = 0
	}
}

func (b *circuitBreaker) push(success bool) {
	if b.filled == b.windowSize {
		if !b.outcomes[b.head] { // the slot we are about to overwrite held a failure
			b.failures--
		}
	} else {
		b.filled++
	}
	b.outcomes[b.head] = success
	if !success {
		b.failures++
	}
	b.head = (b.head + 1) % b.windowSize
}

func (b *circuitBreaker) failureRate() float64 {
	if b.filled == 0 {
		return 0
	}
	return float64(b.failures) / float64(b.filled)
}

func (b *circuitBreaker) reset() {
	b.open = false
	b.failedProbes = 0
	b.head, b.filled, b.failures = 0, 0, 0
	for i := range b.outcomes {
		b.outcomes[i] = false
	}
}

// healthy reports whether the node may currently be served (circuit closed).
func (b *circuitBreaker) healthy() bool { return !b.open }

// shouldRemove reports that the node has failed recovery enough times to be
// dropped from the pool. The Manager still decides whether the node is actually
// removable (never for statically configured nodes).
func (b *circuitBreaker) shouldRemove() bool {
	return b.open && b.failedProbes >= b.maxFailedProbes
}
