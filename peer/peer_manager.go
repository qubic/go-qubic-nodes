package peer

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/qubic/go-qubic-nodes/metrics"
)

// peerFactory builds a Prober for a candidate address. It is the seam that lets
// the Manager's discovery/cleanup logic be tested with fakes instead of peers
// that open real connections.
type peerFactory func(address, port string) Prober

type Manager struct {
	discoveryEnabled bool
	updateInterval   time.Duration
	debug            bool
	fastWarmup       bool

	configuredPeers []string
	peerPort        string

	peerInfoTimeout time.Duration

	reliablePeersCap   int
	reliablePeers      map[string]Prober
	reliablePeersMutex sync.RWMutex
	maxTick            uint32
	lastUpdate         int64

	maxTickAcceptanceThreshold      uint32
	responseTimeAcceptanceThreshold time.Duration

	newPeer peerFactory
	logger  *slog.Logger

	metrics *metrics.NodesServiceMetrics
}

// Option overrides a Manager dependency. Production code uses the defaults; tests
// inject fakes via WithPeerFactory / WithLogger.
type Option func(*Manager)

// WithPeerFactory overrides how the Manager creates peers for discovered
// candidates.
func WithPeerFactory(f peerFactory) Option {
	return func(m *Manager) {
		m.newPeer = f
	}
}

// WithLogger overrides the logger used by the Manager.
func WithLogger(l *slog.Logger) Option {
	return func(m *Manager) {
		m.logger = l
	}
}

type ManagerConfig struct {
	EnableDiscovery bool
	UpdateInterval  time.Duration
	Debug           bool
	FastWarmUp      bool

	SeedPeers []string
	PeerPort  string

	PeerInfoTimeout time.Duration

	MaxPeers                            int
	NetworkTickAcceptanceThreshold      uint32
	PeerResponseTimeAcceptanceThreshold time.Duration
}

func NewPeerManager(config ManagerConfig, m *metrics.NodesServiceMetrics, opts ...Option) *Manager {
	maxPeers := config.MaxPeers
	if !config.EnableDiscovery {
		maxPeers = len(config.SeedPeers)
	}

	trimmed := make([]string, len(config.SeedPeers))
	for i, p := range config.SeedPeers {
		trimmed[i] = strings.TrimSpace(p)
	}

	m.SetConfiguredNodeCount(maxPeers)

	// The debug-gated log calls are emitted at Debug level, which the default
	// slog handler discards. Lower the level so config.Debug actually surfaces
	// them. An explicit WithLogger still wins, since options are applied below.
	logger := slog.Default()
	if config.Debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	manager := &Manager{
		discoveryEnabled:                config.EnableDiscovery,
		updateInterval:                  config.UpdateInterval,
		debug:                           config.Debug,
		fastWarmup:                      config.FastWarmUp,
		configuredPeers:                 trimmed,
		peerPort:                        config.PeerPort,
		peerInfoTimeout:                 config.PeerInfoTimeout,
		reliablePeersCap:                maxPeers,
		reliablePeers:                   make(map[string]Prober),
		reliablePeersMutex:              sync.RWMutex{},
		maxTickAcceptanceThreshold:      config.NetworkTickAcceptanceThreshold,
		responseTimeAcceptanceThreshold: config.PeerResponseTimeAcceptanceThreshold,
		newPeer:                         func(address, port string) Prober { return NewPeer(address, port) },
		logger:                          logger,
		metrics:                         m,
	}

	for _, opt := range opts {
		opt(manager)
	}

	return manager
}

func (m *Manager) Start(ctx context.Context) {
	m.logger.Info("Starting peer manager...")

	m.update(ctx)

	interval := m.updateInterval
	inFastMode := false
	if m.fastWarmup {
		interval = m.updateInterval / 2
		inFastMode = true
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Stopping peer manager.")
			return
		case <-ticker.C:
			warming := m.update(ctx)
			if !m.fastWarmup {
				continue // feature disabled: stay at normal cadence
			}
			switch {
			case warming && !inFastMode:
				ticker.Reset(m.updateInterval / 2)
				inFastMode = true
			case !warming && inFastMode:
				ticker.Reset(m.updateInterval)
				inFastMode = false
			}
		}
	}
}

// Must satisfy what the previous version of the code provided

type ManagerStatus struct {
	MaxTick          uint32
	LastUpdate       int64
	ReliablePeers    []Prober
	MostReliablePeer Prober
}

func (m *Manager) GetStatus() ManagerStatus {
	m.reliablePeersMutex.RLock()
	defer m.reliablePeersMutex.RUnlock()

	reliablePeersList := make([]Prober, 0, len(m.reliablePeers))
	var mostReliablePeer Prober

	for _, peer := range m.reliablePeers {
		reliablePeersList = append(reliablePeersList, peer)
		if mostReliablePeer == nil || peer.GetLastKnownTick() > mostReliablePeer.GetLastKnownTick() {
			mostReliablePeer = peer
		}
	}

	return ManagerStatus{
		MaxTick:          m.maxTick,
		LastUpdate:       m.lastUpdate,
		ReliablePeers:    reliablePeersList,
		MostReliablePeer: mostReliablePeer,
	}
}

func (m *Manager) GetReliablePeersWithMinimumTick(tick uint32) ([]Prober, int64) {
	status := m.GetStatus()
	reliablePeersAtLeastAtMinimum := make([]Prober, 0, len(status.ReliablePeers))

	for _, peer := range status.ReliablePeers {
		if peer.GetLastKnownTick() >= tick {
			reliablePeersAtLeastAtMinimum = append(reliablePeersAtLeastAtMinimum, peer)
		}
	}
	return reliablePeersAtLeastAtMinimum, status.LastUpdate
}

func (m *Manager) GetPeerCap() int {
	return m.reliablePeersCap
}

func (m *Manager) update(ctx context.Context) bool {
	maxTick, peersInfo := m.checkAndCleanPeers(ctx)
	if len(peersInfo) < m.reliablePeersCap {
		maxTick = m.discoverAndAcquirePeers(ctx, maxTick, peersInfo)
	}

	shouldWarmupFast := false

	m.reliablePeersMutex.Lock()

	m.maxTick = maxTick
	m.lastUpdate = time.Now().UTC().Unix()

	m.metrics.SetReliableNodeCount(len(m.reliablePeers))
	m.metrics.SetNetworkTick(maxTick)

	if len(m.reliablePeers) < m.reliablePeersCap/2 {
		shouldWarmupFast = true
	}

	m.reliablePeersMutex.Unlock()

	return shouldWarmupFast
}

func (m *Manager) checkAndCleanPeers(ctx context.Context) (maxTick uint32, peersInfo map[string]Info) {
	m.logger.Info("Checking existing peers and performing cleanup")

	peersInfo = make(map[string]Info)
	peersToRemove := make([]string, 0)

	var mu sync.Mutex
	var wg sync.WaitGroup

	m.reliablePeersMutex.RLock()
	peers := make(map[string]Prober, len(m.reliablePeers))
	for addr, p := range m.reliablePeers {
		peers[addr] = p
	}
	m.reliablePeersMutex.RUnlock()

	for address, peer := range peers {
		wg.Go(func() {
			peerInfo, err := peer.GetPeerInfo(ctx, m.peerInfoTimeout)
			if err != nil {
				m.logger.Info("Peer marked for removal: unreachable", "address", address)

				// Should remove peer on unsuccessful connection
				mu.Lock()
				peersToRemove = append(peersToRemove, address)
				mu.Unlock()
				return
			}
			mu.Lock()
			peersInfo[address] = peerInfo
			mu.Unlock()
		})
	}
	wg.Wait()

	var behindNetwork []string
	maxTick, behindNetwork = selectRemovals(peersInfo, m.maxTickAcceptanceThreshold)
	for _, address := range behindNetwork {
		// Should remove peer if gap is over acceptable threshold
		m.logger.Info("Peer marked for removal: behind network", "address", address)
	}
	peersToRemove = append(peersToRemove, behindNetwork...)

	m.reliablePeersMutex.Lock()
	for _, peerToRemove := range peersToRemove {
		delete(m.reliablePeers, peerToRemove)
		delete(peersInfo, peerToRemove)

		if m.debug {
			m.logger.Debug("Removed peer", "address", peerToRemove)
		}
	}
	m.reliablePeersMutex.Unlock()
	m.logger.Info("Removed unreliable peers", "count", len(peersToRemove))

	return maxTick, peersInfo
}

func (m *Manager) discoverAndAcquirePeers(ctx context.Context, maxTick uint32, peersInfo map[string]Info) uint32 {

	m.logger.Info("Discovering and acquiring new peers")

	absoluteMaxTick := maxTick

	candidates := collectCandidates(m.configuredPeers, peersInfo, m.discoveryEnabled)
	m.logger.Info("Collected peer candidates", "count", len(candidates))

	slotsNeeded := m.reliablePeersCap - len(peersInfo)
	m.logger.Info("Attempting to fill free peer slots", "slots", slotsNeeded)

	for i := 0; i < len(candidates) && slotsNeeded > 0; {
		batchSize := slotsNeeded * 2
		end := min(i+batchSize, len(candidates))
		candidateBatch := candidates[i:end]
		i = end

		// Probe the whole batch concurrently; each result lands in its own slot
		// so no locking is needed while gathering.
		probed := make([]probeResult, len(candidateBatch))
		var wg sync.WaitGroup
		for idx, candidateAddress := range candidateBatch {
			wg.Go(func() {
				candidatePeer := m.newPeer(candidateAddress, m.peerPort)
				candidatePeerInfo, err := candidatePeer.GetPeerInfo(ctx, m.peerInfoTimeout)
				probed[idx] = probeResult{
					address: candidateAddress,
					peer:    candidatePeer,
					info:    candidatePeerInfo,
					err:     err,
				}
			})
		}
		wg.Wait()

		if m.debug {
			m.logProbeResults(probed, maxTick)
		}

		accepted := selectAcceptable(probed, maxTick, m.maxTickAcceptanceThreshold, m.responseTimeAcceptanceThreshold)

		m.reliablePeersMutex.Lock()
		for _, c := range accepted {
			if slotsNeeded == 0 {
				break
			}
			m.reliablePeers[c.info.Address] = c.peer
			if c.info.CurrentTick > absoluteMaxTick {
				absoluteMaxTick = c.info.CurrentTick
			}
			m.logger.Info("New peer selected", "address", c.info.Address, "tick", c.info.CurrentTick, "responseTime", c.info.ResponseTime)
			slotsNeeded--
		}
		m.reliablePeersMutex.Unlock()
	}

	return absoluteMaxTick
}

// logProbeResults emits the per-candidate debug lines the discovery loop used to
// log inline. Kept separate so selectAcceptable stays a pure decision.
func (m *Manager) logProbeResults(probed []probeResult, maxTick uint32) {
	for _, r := range probed {
		switch {
		case r.err != nil:
			m.logger.Debug("Rejected candidate: unreachable", "address", r.address, "error", r.err)
		case !candidateAcceptable(r.info, maxTick, m.maxTickAcceptanceThreshold, m.responseTimeAcceptanceThreshold):
			m.logger.Debug("Rejected candidate: unreliable", "address", r.address, "tick", r.info.CurrentTick, "responseTime", r.info.ResponseTime)
		default:
			m.logger.Debug("Suitable candidate found", "address", r.info.Address, "tick", r.info.CurrentTick, "responseTime", r.info.ResponseTime)
		}
	}
}

func acceptable(peerTick, maxTick, threshold uint32) bool {
	return peerTick+threshold >= maxTick
}

// probeResult is the outcome of probing a single candidate address.
type probeResult struct {
	address string
	peer    Prober
	info    Info
	err     error
}

// selectRemovals computes the network max tick from the reachable peer infos and
// returns the addresses that have fallen behind the network by more than
// threshold. Unreachable peers are handled separately by the caller.
func selectRemovals(infos map[string]Info, threshold uint32) (maxTick uint32, behindNetwork []string) {
	for _, info := range infos {
		if info.CurrentTick > maxTick {
			maxTick = info.CurrentTick
		}
	}

	for address, info := range infos {
		if !acceptable(info.CurrentTick, maxTick, threshold) {
			behindNetwork = append(behindNetwork, address)
		}
	}

	return maxTick, behindNetwork
}

// collectCandidates builds the ordered, de-duplicated list of addresses to probe
// for new peers. Addresses already present in peersInfo are excluded, configured
// peers come first, and discovered peers are appended only when discovery is on.
func collectCandidates(configured []string, peersInfo map[string]Info, discoveryEnabled bool) []string {
	seen := make(map[string]struct{})
	for address := range peersInfo {
		seen[address] = struct{}{}
	}

	candidates := make([]string, 0)
	addCandidate := func(addr string) {
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		candidates = append(candidates, addr)
	}

	for _, addr := range configured {
		addCandidate(addr)
	}

	if discoveryEnabled {
		for _, peerInfo := range peersInfo {
			for _, addr := range peerInfo.Peers {
				addCandidate(addr)
			}
		}
	}

	return candidates
}

// candidateAcceptable reports whether a probed candidate is close enough to the
// network tip and responsive enough to be selected.
func candidateAcceptable(info Info, maxTick, tickThreshold uint32, rtThreshold time.Duration) bool {
	return acceptable(info.CurrentTick, maxTick, tickThreshold) && info.ResponseTime <= rtThreshold
}

// selectAcceptable keeps the reachable, reliable candidates from a probed batch
// and returns them sorted fastest-response-time first.
func selectAcceptable(probed []probeResult, maxTick, tickThreshold uint32, rtThreshold time.Duration) []probeResult {
	accepted := make([]probeResult, 0, len(probed))
	for _, r := range probed {
		if r.err != nil {
			continue
		}
		if !candidateAcceptable(r.info, maxTick, tickThreshold, rtThreshold) {
			continue
		}
		accepted = append(accepted, r)
	}

	slices.SortFunc(accepted, func(a, b probeResult) int {
		return cmp.Compare(a.info.ResponseTime, b.info.ResponseTime)
	})

	return accepted
}
