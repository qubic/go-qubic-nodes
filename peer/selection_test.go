package peer

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcceptable(t *testing.T) {
	testData := []struct {
		name      string
		peerTick  uint32
		maxTick   uint32
		threshold uint32
		want      bool
	}{
		{name: "peer at the network tip", peerTick: 1000, maxTick: 1000, threshold: 30, want: true},
		{name: "peer ahead of known max tick", peerTick: 1005, maxTick: 1000, threshold: 30, want: true},
		{name: "peer exactly at the threshold", peerTick: 970, maxTick: 1000, threshold: 30, want: true},
		{name: "peer one tick past the threshold", peerTick: 969, maxTick: 1000, threshold: 30, want: false},
		{name: "zero threshold requires the exact tip", peerTick: 999, maxTick: 1000, threshold: 0, want: false},
		// Cold start: nothing is known yet, so every peer looks acceptable. This is
		// intentional at the pure-function level; see the cold-start behaviour of
		// discoverAndAcquirePeers for what that means in practice.
		{name: "unknown max tick accepts anything", peerTick: 1, maxTick: 0, threshold: 30, want: true},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, acceptable(test.peerTick, test.maxTick, test.threshold))
		})
	}

	// Documents a known limitation rather than desired behaviour: peerTick+threshold
	// wraps around, so a peer sitting exactly at the tip is judged to be behind it.
	// Unreachable with real tick numbers, but pinned so the behaviour cannot change
	// silently if the comparison is ever rewritten.
	t.Run("known limitation: uint32 overflow near the ceiling", func(t *testing.T) {
		assert.False(t, acceptable(math.MaxUint32, math.MaxUint32, 30))
	})
}

func TestSelectRemovals(t *testing.T) {
	t.Run("no peer infos yields no max tick and no removals", func(t *testing.T) {
		maxTick, behindNetwork := selectRemovals(map[string]Info{}, 30)

		assert.Zero(t, maxTick)
		assert.Empty(t, behindNetwork)
	})

	t.Run("a single peer defines the max tick and always survives", func(t *testing.T) {
		infos := map[string]Info{
			"1.2.3.4": {Address: "1.2.3.4", CurrentTick: 1000},
		}

		maxTick, behindNetwork := selectRemovals(infos, 30)

		assert.Equal(t, uint32(1000), maxTick)
		assert.Empty(t, behindNetwork)
	})

	t.Run("returns only the peers that fell behind the threshold", func(t *testing.T) {
		infos := map[string]Info{
			"1.2.3.4": {Address: "1.2.3.4", CurrentTick: 1000}, // the tip
			"2.3.4.5": {Address: "2.3.4.5", CurrentTick: 985},  // within threshold
			"3.4.5.6": {Address: "3.4.5.6", CurrentTick: 970},  // exactly at threshold
			"4.5.6.7": {Address: "4.5.6.7", CurrentTick: 969},  // one past
			"5.6.7.8": {Address: "5.6.7.8", CurrentTick: 100},  // far behind
		}

		maxTick, behindNetwork := selectRemovals(infos, 30)

		assert.Equal(t, uint32(1000), maxTick)
		assert.ElementsMatch(t, []string{"4.5.6.7", "5.6.7.8"}, behindNetwork)
	})
}

func TestCollectCandidates(t *testing.T) {
	t.Run("configured peers are probed first and in order", func(t *testing.T) {
		configured := []string{"1.2.3.4", "2.3.4.5", "3.4.5.6"}

		candidates := collectCandidates(configured, map[string]Info{}, true)

		assert.Equal(t, configured, candidates)
	})

	t.Run("peers already held are not candidates again", func(t *testing.T) {
		configured := []string{"1.2.3.4", "2.3.4.5"}
		peersInfo := map[string]Info{
			"1.2.3.4": {Address: "1.2.3.4"},
		}

		candidates := collectCandidates(configured, peersInfo, true)

		assert.Equal(t, []string{"2.3.4.5"}, candidates)
	})

	t.Run("discovered peers are appended after configured ones", func(t *testing.T) {
		configured := []string{"1.2.3.4"}
		peersInfo := map[string]Info{
			"9.9.9.9": {Address: "9.9.9.9", Peers: []string{"2.3.4.5", "3.4.5.6"}},
		}

		candidates := collectCandidates(configured, peersInfo, true)

		require.Len(t, candidates, 3)
		assert.Equal(t, "1.2.3.4", candidates[0], "configured peers must come first")
		assert.ElementsMatch(t, []string{"2.3.4.5", "3.4.5.6"}, candidates[1:])
	})

	t.Run("duplicates across peers are collapsed", func(t *testing.T) {
		configured := []string{"1.2.3.4"}
		peersInfo := map[string]Info{
			"9.9.9.9": {Address: "9.9.9.9", Peers: []string{"2.3.4.5", "3.4.5.6"}},
			"8.8.8.8": {Address: "8.8.8.8", Peers: []string{"2.3.4.5", "1.2.3.4"}},
		}

		candidates := collectCandidates(configured, peersInfo, true)

		assert.ElementsMatch(t, []string{"1.2.3.4", "2.3.4.5", "3.4.5.6"}, candidates)
	})

	t.Run("discovery disabled falls back to configured peers only", func(t *testing.T) {
		configured := []string{"1.2.3.4"}
		peersInfo := map[string]Info{
			"9.9.9.9": {Address: "9.9.9.9", Peers: []string{"2.3.4.5", "3.4.5.6"}},
		}

		candidates := collectCandidates(configured, peersInfo, false)

		assert.Equal(t, []string{"1.2.3.4"}, candidates)
	})
}

func TestCandidateAcceptable(t *testing.T) {
	const maxTick, tickThreshold = uint32(1000), uint32(30)
	const rtThreshold = 250 * time.Millisecond

	testData := []struct {
		name string
		info Info
		want bool
	}{
		{
			name: "in sync and fast",
			info: Info{CurrentTick: 1000, ResponseTime: 10 * time.Millisecond},
			want: true,
		},
		{
			name: "in sync but too slow",
			info: Info{CurrentTick: 1000, ResponseTime: 251 * time.Millisecond},
			want: false,
		},
		{
			name: "fast but behind the network",
			info: Info{CurrentTick: 900, ResponseTime: 10 * time.Millisecond},
			want: false,
		},
		{
			name: "response time exactly at the threshold is accepted",
			info: Info{CurrentTick: 1000, ResponseTime: rtThreshold},
			want: true,
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			got := candidateAcceptable(test.info, maxTick, tickThreshold, rtThreshold)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestSelectAcceptable(t *testing.T) {
	const maxTick, tickThreshold = uint32(1000), uint32(30)
	const rtThreshold = 250 * time.Millisecond

	t.Run("no candidates yields no selection", func(t *testing.T) {
		assert.Empty(t, selectAcceptable(nil, maxTick, tickThreshold, rtThreshold))
	})

	t.Run("unreachable and unreliable candidates are dropped", func(t *testing.T) {
		probed := []probeResult{
			{address: "1.2.3.4", err: errors.New("connection refused")},
			{address: "2.3.4.5", info: Info{Address: "2.3.4.5", CurrentTick: 900, ResponseTime: time.Millisecond}},
			{address: "3.4.5.6", info: Info{Address: "3.4.5.6", CurrentTick: 1000, ResponseTime: time.Second}},
			{address: "4.5.6.7", info: Info{Address: "4.5.6.7", CurrentTick: 1000, ResponseTime: time.Millisecond}},
		}

		accepted := selectAcceptable(probed, maxTick, tickThreshold, rtThreshold)

		require.Len(t, accepted, 1)
		assert.Equal(t, "4.5.6.7", accepted[0].address)
	})

	t.Run("accepted candidates are ordered fastest first", func(t *testing.T) {
		probed := []probeResult{
			{address: "slow", info: Info{Address: "slow", CurrentTick: 1000, ResponseTime: 200 * time.Millisecond}},
			{address: "fast", info: Info{Address: "fast", CurrentTick: 1000, ResponseTime: 5 * time.Millisecond}},
			{address: "medium", info: Info{Address: "medium", CurrentTick: 1000, ResponseTime: 50 * time.Millisecond}},
		}

		accepted := selectAcceptable(probed, maxTick, tickThreshold, rtThreshold)

		require.Len(t, accepted, 3)
		assert.Equal(t, []string{"fast", "medium", "slow"}, addressesOf(accepted))
	})
}

func addressesOf(results []probeResult) []string {
	addresses := make([]string, 0, len(results))
	for _, r := range results {
		addresses = append(addresses, r.address)
	}
	return addresses
}
