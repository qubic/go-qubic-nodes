package peer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/qubic/go-node-connector/v2/types"
	"github.com/qubic/go-qubic-nodes/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePeer is a Prober that answers from canned data instead of dialing a node.
// A nil err means the probe succeeds.
type fakePeer struct {
	address      string
	port         string
	tick         uint32
	peers        types.PublicPeers
	responseTime time.Duration
	err          error
}

func (f *fakePeer) GetPeerInfo(context.Context, time.Duration) (Info, error) {
	if f.err != nil {
		return Info{}, f.err
	}
	return Info{
		Address:      f.address,
		Port:         f.port,
		CurrentTick:  f.tick,
		Peers:        f.peers,
		ResponseTime: f.responseTime,
	}, nil
}

func (f *fakePeer) GetAddress() string                   { return f.address }
func (f *fakePeer) GetPort() string                      { return f.port }
func (f *fakePeer) GetLastKnownTick() uint32             { return f.tick }
func (f *fakePeer) GetLastKnownPeers() types.PublicPeers { return f.peers }

// newTestManager builds a Manager wired to a peer factory that serves the given
// fakes by address. Candidates with no entry in fakes are treated as unreachable.
// It also returns a counter of how many candidates were actually probed.
func newTestManager(t *testing.T, config ManagerConfig, fakes map[string]*fakePeer) (*Manager, *atomic.Int64) {
	t.Helper()

	var probes atomic.Int64
	factory := func(address, port string) Prober {
		probes.Add(1)
		if fake, ok := fakes[address]; ok {
			return fake
		}
		return &fakePeer{address: address, port: port, err: errors.New("unreachable")}
	}

	m := metrics.NewNodesServiceMetrics(prometheus.NewRegistry(), "test")
	manager := NewPeerManager(config, m,
		WithPeerFactory(factory),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)

	return manager, &probes
}

// proberMap turns fakes into the map layout Manager keeps internally.
func proberMap(fakes ...*fakePeer) map[string]Prober {
	peers := make(map[string]Prober, len(fakes))
	for _, f := range fakes {
		peers[f.address] = f
	}
	return peers
}

func TestCheckAndCleanPeers(t *testing.T) {
	t.Run("unreachable peers are dropped and reachable ones reported", func(t *testing.T) {
		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                true,
			MaxPeers:                       10,
			NetworkTickAcceptanceThreshold: 30,
		}, nil)

		manager.reliablePeers = proberMap(
			&fakePeer{address: "1.2.3.4", tick: 1000},
			&fakePeer{address: "2.3.4.5", err: errors.New("connection refused")},
			&fakePeer{address: "3.4.5.6", tick: 998},
		)

		maxTick, peersInfo := manager.checkAndCleanPeers(t.Context())

		assert.Equal(t, uint32(1000), maxTick)
		assert.ElementsMatch(t, []string{"1.2.3.4", "3.4.5.6"}, keysOf(peersInfo))
		assert.ElementsMatch(t, []string{"1.2.3.4", "3.4.5.6"}, keysOf(manager.reliablePeers))
	})

	t.Run("peers that fell behind the network are dropped", func(t *testing.T) {
		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                true,
			MaxPeers:                       10,
			NetworkTickAcceptanceThreshold: 30,
		}, nil)

		manager.reliablePeers = proberMap(
			&fakePeer{address: "1.2.3.4", tick: 1000}, // the tip
			&fakePeer{address: "2.3.4.5", tick: 970},  // exactly at threshold
			&fakePeer{address: "3.4.5.6", tick: 500},  // far behind
		)

		maxTick, peersInfo := manager.checkAndCleanPeers(t.Context())

		assert.Equal(t, uint32(1000), maxTick)
		assert.ElementsMatch(t, []string{"1.2.3.4", "2.3.4.5"}, keysOf(manager.reliablePeers))
		// A dropped peer must also disappear from peersInfo, otherwise discovery
		// would count it as an occupied slot.
		assert.ElementsMatch(t, []string{"1.2.3.4", "2.3.4.5"}, keysOf(peersInfo))
	})

	// A total outage wipes the peer set, and with it the only source of tick
	// information the Manager has. The reported max tick therefore collapses to
	// zero rather than holding the last known value. See TestUpdate_totalOutage
	// for what that means end to end.
	t.Run("losing every peer resets the max tick to zero", func(t *testing.T) {
		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                true,
			MaxPeers:                       10,
			NetworkTickAcceptanceThreshold: 30,
		}, nil)

		manager.reliablePeers = proberMap(
			&fakePeer{address: "1.2.3.4", err: errors.New("connection refused")},
			&fakePeer{address: "2.3.4.5", err: errors.New("connection refused")},
		)
		manager.maxTick = 1000

		maxTick, peersInfo := manager.checkAndCleanPeers(t.Context())

		assert.Zero(t, maxTick, "no reachable peer means no tick information at all")
		assert.Empty(t, peersInfo)
		assert.Empty(t, manager.reliablePeers)
	})
}

// TestUpdate_totalOutage pins what the service reports when every peer, seed
// included, is unreachable: the status endpoint serves a max tick of zero, not
// the last known good value.
func TestUpdate_totalOutage(t *testing.T) {
	manager, _ := newTestManager(t, ManagerConfig{
		EnableDiscovery:                     true,
		SeedPeers:                           []string{"1.1.1.1", "2.2.2.2"},
		MaxPeers:                            10,
		NetworkTickAcceptanceThreshold:      30,
		PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
	}, nil) // no fakes, so every address the factory produces is unreachable

	// The peer held from the previous cycle has now gone dark too.
	manager.reliablePeers = proberMap(
		&fakePeer{address: "9.9.9.9", tick: 1000, err: errors.New("connection refused")},
	)
	manager.maxTick = 1000

	warming := manager.update(t.Context())

	status := manager.GetStatus()
	assert.Empty(t, status.ReliablePeers)
	assert.Nil(t, status.MostReliablePeer)
	assert.Zero(t, status.MaxTick, "max tick is not carried over across a total outage")
	assert.True(t, warming, "an empty peer set must ask for the fast warmup cadence")
}

// TestDiscoverAndAcquirePeers_coldStart covers the first update after startup,
// when the Manager holds no peers and therefore has no reference tick. Every
// candidate is judged against a max tick of zero, so acceptance is decided purely
// on response time.
func TestDiscoverAndAcquirePeers_coldStart(t *testing.T) {
	t.Run("a peer far behind the network is accepted when nothing is known yet", func(t *testing.T) {
		fakes := map[string]*fakePeer{
			"1.1.1.1": {address: "1.1.1.1", tick: 1000, responseTime: 100 * time.Millisecond},
			"2.2.2.2": {address: "2.2.2.2", tick: 10, responseTime: time.Millisecond},
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           []string{"1.1.1.1", "2.2.2.2"},
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		maxTick := manager.discoverAndAcquirePeers(t.Context(), 0, map[string]Info{})

		// 2.2.2.2 is 990 ticks behind and is taken anyway, because the threshold is
		// applied against a max tick of zero rather than against the best tick seen
		// in the batch.
		assert.ElementsMatch(t, []string{"1.1.1.1", "2.2.2.2"}, keysOf(manager.reliablePeers))
		assert.Equal(t, uint32(1000), maxTick, "the highest tick seen is still reported")
	})

	t.Run("the following update evicts what cold start let in", func(t *testing.T) {
		fakes := map[string]*fakePeer{
			"1.1.1.1": {address: "1.1.1.1", tick: 1000, responseTime: 100 * time.Millisecond},
			"2.2.2.2": {address: "2.2.2.2", tick: 10, responseTime: time.Millisecond},
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           []string{"1.1.1.1", "2.2.2.2"},
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		manager.update(t.Context()) // cold start: takes both
		require.Len(t, manager.reliablePeers, 2)

		manager.update(t.Context()) // now there is a reference tick to judge against

		assert.ElementsMatch(t, []string{"1.1.1.1"}, keysOf(manager.reliablePeers),
			"the lagging peer should be evicted once a max tick is known")
		assert.Equal(t, uint32(1000), manager.GetStatus().MaxTick)
	})

	t.Run("a lone lagging peer is never recognised as lagging", func(t *testing.T) {
		// Only the slow, far-behind peer is reachable. It defines the max tick by
		// itself, so it is always within threshold of "the network" and survives
		// every cleanup cycle.
		fakes := map[string]*fakePeer{
			"2.2.2.2": {address: "2.2.2.2", tick: 10, responseTime: time.Millisecond},
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           []string{"1.1.1.1", "2.2.2.2"},
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		for range 3 {
			manager.update(t.Context())
		}

		assert.ElementsMatch(t, []string{"2.2.2.2"}, keysOf(manager.reliablePeers))
		assert.Equal(t, uint32(10), manager.GetStatus().MaxTick,
			"the served max tick is only ever as good as the peers currently held")
	})
}

func TestDiscoverAndAcquirePeers(t *testing.T) {
	t.Run("fills up to the cap and no further", func(t *testing.T) {
		fakes := make(map[string]*fakePeer)
		seeds := make([]string, 0, 50)
		for i := range 50 {
			address := fmt.Sprintf("10.0.0.%d", i)
			seeds = append(seeds, address)
			fakes[address] = &fakePeer{address: address, tick: 1000, responseTime: time.Millisecond}
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           seeds,
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		manager.discoverAndAcquirePeers(t.Context(), 1000, map[string]Info{})

		assert.Len(t, manager.reliablePeers, 10)
	})

	t.Run("prefers the fastest candidates in a batch", func(t *testing.T) {
		// 20 candidates and a cap of 10 means the first batch (slots*2) covers every
		// candidate, so the selection here is over the whole set.
		fakes := make(map[string]*fakePeer)
		seeds := make([]string, 0, 20)
		for i := range 20 {
			address := fmt.Sprintf("10.0.0.%d", i)
			seeds = append(seeds, address)
			fakes[address] = &fakePeer{
				address:      address,
				tick:         1000,
				responseTime: time.Duration(i+1) * time.Millisecond,
			}
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           seeds,
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: time.Second,
		}, fakes)

		manager.discoverAndAcquirePeers(t.Context(), 1000, map[string]Info{})

		require.Len(t, manager.reliablePeers, 10)
		for i := range 10 {
			assert.Contains(t, manager.reliablePeers, fmt.Sprintf("10.0.0.%d", i),
				"the ten fastest candidates should have been selected")
		}
	})

	t.Run("unreachable and unreliable candidates do not consume slots", func(t *testing.T) {
		fakes := map[string]*fakePeer{
			"1.1.1.1": {address: "1.1.1.1", tick: 1000, responseTime: time.Millisecond},
			// 2.2.2.2 is absent, so the factory makes it unreachable.
			"3.3.3.3": {address: "3.3.3.3", tick: 500, responseTime: time.Millisecond}, // behind
			"4.4.4.4": {address: "4.4.4.4", tick: 1000, responseTime: 5 * time.Second}, // too slow
			"5.5.5.5": {address: "5.5.5.5", tick: 1000, responseTime: 2 * time.Millisecond},
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"},
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		manager.discoverAndAcquirePeers(t.Context(), 1000, map[string]Info{})

		assert.ElementsMatch(t, []string{"1.1.1.1", "5.5.5.5"}, keysOf(manager.reliablePeers))
	})

	t.Run("returns the highest tick seen while acquiring", func(t *testing.T) {
		fakes := map[string]*fakePeer{
			"1.1.1.1": {address: "1.1.1.1", tick: 1000, responseTime: time.Millisecond},
			"2.2.2.2": {address: "2.2.2.2", tick: 1020, responseTime: time.Millisecond},
		}

		manager, _ := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           []string{"1.1.1.1", "2.2.2.2"},
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		maxTick := manager.discoverAndAcquirePeers(t.Context(), 1000, map[string]Info{})

		assert.Equal(t, uint32(1020), maxTick)
	})

	// The regression guard for the old recursive peer discovery, which spawned an
	// unbounded number of goroutines and could block forever on a full channel.
	// Discovery must terminate promptly and probe only a bounded prefix of the
	// candidate list rather than the whole thing.
	t.Run("a huge candidate list terminates and stays bounded", func(t *testing.T) {
		fakes := make(map[string]*fakePeer)
		seeds := make([]string, 0, 500)
		for i := range 500 {
			address := fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256)
			seeds = append(seeds, address)
			fakes[address] = &fakePeer{address: address, tick: 1000, responseTime: time.Millisecond}
		}

		manager, probes := newTestManager(t, ManagerConfig{
			EnableDiscovery:                     true,
			SeedPeers:                           seeds,
			MaxPeers:                            10,
			NetworkTickAcceptanceThreshold:      30,
			PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
		}, fakes)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			manager.discoverAndAcquirePeers(ctx, 1000, map[string]Info{})
		}()

		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("discovery did not terminate within 5s")
		}

		assert.Len(t, manager.reliablePeers, 10)
		assert.LessOrEqual(t, probes.Load(), int64(20),
			"only the first batch of slots*2 candidates should have been probed")
	})
}

func TestGetStatus(t *testing.T) {
	t.Run("reports no peers and no most reliable peer when empty", func(t *testing.T) {
		manager, _ := newTestManager(t, ManagerConfig{MaxPeers: 10}, nil)

		status := manager.GetStatus()

		assert.Empty(t, status.ReliablePeers)
		// The web handler dereferences this, so nil is load bearing.
		assert.Nil(t, status.MostReliablePeer)
	})

	t.Run("the most reliable peer is the one at the highest tick", func(t *testing.T) {
		manager, _ := newTestManager(t, ManagerConfig{MaxPeers: 10}, nil)
		manager.reliablePeers = proberMap(
			&fakePeer{address: "1.2.3.4", tick: 990},
			&fakePeer{address: "2.3.4.5", tick: 1000},
			&fakePeer{address: "3.4.5.6", tick: 995},
		)
		manager.maxTick = 1000
		manager.lastUpdate = 42

		status := manager.GetStatus()

		assert.Len(t, status.ReliablePeers, 3)
		assert.Equal(t, uint32(1000), status.MaxTick)
		assert.Equal(t, int64(42), status.LastUpdate)
		require.NotNil(t, status.MostReliablePeer)
		assert.Equal(t, "2.3.4.5", status.MostReliablePeer.GetAddress())
	})
}

func TestGetReliablePeersWithMinimumTick(t *testing.T) {
	testData := []struct {
		name        string
		ticks       map[string]uint32
		minimumTick uint32
		want        []string
	}{
		{
			name:        "no peers at all",
			ticks:       map[string]uint32{},
			minimumTick: 1993,
			want:        []string{},
		},
		{
			name:        "single peer below the minimum",
			ticks:       map[string]uint32{"1.2.3.4": 1992},
			minimumTick: 1993,
			want:        []string{},
		},
		{
			name:        "all peers below the minimum",
			ticks:       map[string]uint32{"1.2.3.4": 1992, "2.3.4.5": 1991},
			minimumTick: 1993,
			want:        []string{},
		},
		{
			name:        "one peer above the minimum",
			ticks:       map[string]uint32{"1.2.3.4": 1992, "2.3.4.5": 1991, "3.4.5.6": 1994},
			minimumTick: 1993,
			want:        []string{"3.4.5.6"},
		},
		{
			name:        "a peer exactly at the minimum is included",
			ticks:       map[string]uint32{"1.2.3.4": 1992, "2.3.4.5": 1993},
			minimumTick: 1993,
			want:        []string{"2.3.4.5"},
		},
		{
			name:        "peers at and above the minimum",
			ticks:       map[string]uint32{"1.2.3.4": 1992, "2.3.4.5": 1993, "3.4.5.6": 1994},
			minimumTick: 1993,
			want:        []string{"2.3.4.5", "3.4.5.6"},
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			manager, _ := newTestManager(t, ManagerConfig{MaxPeers: 10}, nil)
			manager.reliablePeers = make(map[string]Prober, len(test.ticks))
			for address, tick := range test.ticks {
				manager.reliablePeers[address] = &fakePeer{address: address, tick: tick}
			}
			manager.lastUpdate = 42

			peers, lastUpdate := manager.GetReliablePeersWithMinimumTick(test.minimumTick)

			addresses := make([]string, 0, len(peers))
			for _, p := range peers {
				addresses = append(addresses, p.GetAddress())
			}
			assert.ElementsMatch(t, test.want, addresses)
			assert.Equal(t, int64(42), lastUpdate)
		})
	}
}

// TestConcurrentReadsDuringUpdate is meaningful under -race. The previous
// implementation mutated its peer list from a bare goroutine while HTTP handlers
// read it, so this guards the lock discipline that replaced it.
func TestConcurrentReadsDuringUpdate(t *testing.T) {
	fakes := make(map[string]*fakePeer)
	seeds := make([]string, 0, 20)
	for i := range 20 {
		address := fmt.Sprintf("10.0.0.%d", i)
		seeds = append(seeds, address)
		fakes[address] = &fakePeer{address: address, tick: uint32(1000 + i), responseTime: time.Millisecond}
	}

	manager, _ := newTestManager(t, ManagerConfig{
		EnableDiscovery:                     true,
		SeedPeers:                           seeds,
		MaxPeers:                            10,
		NetworkTickAcceptanceThreshold:      30,
		PeerResponseTimeAcceptanceThreshold: 250 * time.Millisecond,
	}, fakes)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup

	wg.Go(func() {
		for range 50 {
			manager.update(ctx)
		}
		cancel()
	})

	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					manager.GetStatus()
					manager.GetReliablePeersWithMinimumTick(1000)
					manager.GetPeerCap()
				}
			}
		})
	}

	wg.Wait()
}

func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

var _ = prometheus.NewRegistry
