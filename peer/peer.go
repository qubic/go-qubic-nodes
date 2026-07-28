package peer

import (
	"context"
	"fmt"
	"sync"
	"time"

	qubic "github.com/qubic/go-node-connector/v2"
	"github.com/qubic/go-node-connector/v2/types"
)

// Prober is the behaviour the Manager depends on. Both the real *Peer and test
// doubles satisfy it, so the Manager can be exercised without any network I/O.
type Prober interface {
	GetPeerInfo(ctx context.Context, timeout time.Duration) (Info, error)
	GetAddress() string
	GetPort() string
	GetLastKnownTick() uint32
	GetLastKnownPeers() types.PublicPeers
}

// nodeConn is the subset of the qubic client that Peer needs. Keeping it small
// makes it cheap to fake in tests.
type nodeConn interface {
	GetTickInfo(ctx context.Context) (types.TickInfo, error)
	GetPeers() types.PublicPeers
	Close() error
}

// connectFunc dials a node and returns a connection. It is the seam that lets
// GetPeerInfo be tested without opening a real socket.
type connectFunc func(ctx context.Context, address, port string) (nodeConn, error)

// defaultConnect builds the production dialer, backed by the qubic client. With
// noPeerFetching set, the client skips the peer exchange, so GetPeers reports an
// empty list.
func defaultConnect(noPeerFetching bool) connectFunc {
	return func(ctx context.Context, address, port string) (nodeConn, error) {
		var opts []qubic.Option
		if noPeerFetching {
			opts = append(opts, qubic.WithoutPeers())
		}

		client, err := qubic.NewClient(ctx, address, port, opts...)
		if err != nil {
			return nil, err
		}
		return &qubicConn{client}, nil
	}
}

// qubicConn adapts *qubic.Client (which exposes Peers as a field) to nodeConn.
type qubicConn struct {
	*qubic.Client
}

func (c *qubicConn) GetPeers() types.PublicPeers {
	return c.Peers
}

type Peer struct {
	address        string
	port           string
	lastKnownTick  uint32
	lastKnownPeers types.PublicPeers

	connect connectFunc

	mu sync.RWMutex
}

type Info struct {
	Address      string
	Port         string
	CurrentTick  uint32
	Peers        types.PublicPeers
	ResponseTime time.Duration
}

// NewPeer creates a peer. noPeerFetching skips the peer exchange when dialing,
// which is what the Manager wants when discovery is disabled.
func NewPeer(address, port string, noPeerFetching bool) *Peer {
	return &Peer{
		address: address,
		port:    port,
		connect: defaultConnect(noPeerFetching),
	}
}

func (p *Peer) GetPeerInfo(ctx context.Context, timeout time.Duration) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	client, err := p.connect(ctx, p.address, p.port)
	if err != nil {
		return Info{}, fmt.Errorf("creating peer connection: %w", err)
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		return Info{}, fmt.Errorf("getting peer tick info: %w", err)
	}

	elapsed := time.Since(start)

	peers := client.GetPeers()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.lastKnownTick = tickInfo.Tick
	p.lastKnownPeers = peers

	return Info{
		Address:      p.address,
		Port:         p.port,
		CurrentTick:  tickInfo.Tick,
		Peers:        peers,
		ResponseTime: elapsed,
	}, nil
}

func (p *Peer) GetAddress() string {
	return p.address
}

func (p *Peer) GetPort() string {
	return p.port
}

func (p *Peer) GetLastKnownTick() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastKnownTick
}

func (p *Peer) GetLastKnownPeers() types.PublicPeers {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastKnownPeers
}
