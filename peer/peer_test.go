package peer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qubic/go-node-connector/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConn is a nodeConn that answers from canned data instead of talking to a
// node. delay lets a test make GetTickInfo outlast the probe timeout.
type fakeConn struct {
	tick    uint32
	peers   types.PublicPeers
	tickErr error
	delay   time.Duration

	closed atomic.Bool
}

func (c *fakeConn) GetTickInfo(ctx context.Context) (types.TickInfo, error) {
	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-ctx.Done():
			return types.TickInfo{}, ctx.Err()
		}
	}
	if c.tickErr != nil {
		return types.TickInfo{}, c.tickErr
	}
	return types.TickInfo{Tick: c.tick}, nil
}

func (c *fakeConn) GetPeers() types.PublicPeers { return c.peers }

func (c *fakeConn) Close() error {
	c.closed.Store(true)
	return nil
}

// newTestPeer builds a Peer whose dialer is replaced by connect, bypassing the
// network entirely.
func newTestPeer(address, port string, connect connectFunc) *Peer {
	p := NewPeer(address, port)
	p.connect = connect
	return p
}

func TestPeer_GetPeerInfo(t *testing.T) {
	t.Run("a successful probe reports the tick, peers and a measured response time", func(t *testing.T) {
		conn := &fakeConn{tick: 1234, peers: types.PublicPeers{"2.2.2.2", "3.3.3.3"}, delay: 5 * time.Millisecond}
		p := newTestPeer("1.1.1.1", "21841", func(context.Context, string, string) (nodeConn, error) {
			return conn, nil
		})

		info, err := p.GetPeerInfo(t.Context(), time.Second)
		require.NoError(t, err)

		assert.Equal(t, "1.1.1.1", info.Address)
		assert.Equal(t, "21841", info.Port)
		assert.Equal(t, uint32(1234), info.CurrentTick)
		assert.Equal(t, types.PublicPeers{"2.2.2.2", "3.3.3.3"}, info.Peers)
		assert.GreaterOrEqual(t, info.ResponseTime, 5*time.Millisecond,
			"response time must cover the time spent waiting on the node")
		assert.True(t, conn.closed.Load(), "the connection must be closed after a probe")
	})

	t.Run("a successful probe caches the tick and peers on the peer", func(t *testing.T) {
		conn := &fakeConn{tick: 1234, peers: types.PublicPeers{"2.2.2.2"}}
		p := newTestPeer("1.1.1.1", "21841", func(context.Context, string, string) (nodeConn, error) {
			return conn, nil
		})

		_, err := p.GetPeerInfo(t.Context(), time.Second)
		require.NoError(t, err)

		assert.Equal(t, uint32(1234), p.GetLastKnownTick())
		assert.Equal(t, types.PublicPeers{"2.2.2.2"}, p.GetLastKnownPeers())
	})

	t.Run("a dial failure is reported and leaves the cached state untouched", func(t *testing.T) {
		p := newTestPeer("1.1.1.1", "21841", func(context.Context, string, string) (nodeConn, error) {
			return nil, errors.New("connection refused")
		})
		p.lastKnownTick = 999
		p.lastKnownPeers = types.PublicPeers{"9.9.9.9"}

		info, err := p.GetPeerInfo(t.Context(), time.Second)
		require.ErrorContains(t, err, "connection refused")
		assert.Zero(t, info)

		// A failed probe must not erase what we last knew: the Manager keeps
		// serving these values until it decides to drop the peer.
		assert.Equal(t, uint32(999), p.GetLastKnownTick())
		assert.Equal(t, types.PublicPeers{"9.9.9.9"}, p.GetLastKnownPeers())
	})

	t.Run("a tick info failure is reported, closes the connection and leaves the cached state untouched", func(t *testing.T) {
		conn := &fakeConn{tickErr: errors.New("node is not responding")}
		p := newTestPeer("1.1.1.1", "21841", func(context.Context, string, string) (nodeConn, error) {
			return conn, nil
		})
		p.lastKnownTick = 999

		_, err := p.GetPeerInfo(t.Context(), time.Second)
		require.ErrorContains(t, err, "node is not responding")

		assert.True(t, conn.closed.Load(), "the connection must be closed even when the probe fails")
		assert.Equal(t, uint32(999), p.GetLastKnownTick())
	})

	t.Run("the timeout bounds a node that never answers", func(t *testing.T) {
		conn := &fakeConn{tick: 1234, delay: time.Minute}
		p := newTestPeer("1.1.1.1", "21841", func(context.Context, string, string) (nodeConn, error) {
			return conn, nil
		})

		start := time.Now()
		_, err := p.GetPeerInfo(t.Context(), 20*time.Millisecond)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), time.Second, "the probe must give up at the timeout, not wait for the node")
	})

	t.Run("cancelling the parent context aborts the probe", func(t *testing.T) {
		conn := &fakeConn{tick: 1234, delay: time.Minute}
		p := newTestPeer("1.1.1.1", "21841", func(context.Context, string, string) (nodeConn, error) {
			return conn, nil
		})

		// The Manager passes its root context down, so a shutdown must unwind
		// probes that are still waiting on a node.
		ctx, cancel := context.WithCancel(t.Context())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_, err := p.GetPeerInfo(ctx, time.Minute)
		require.ErrorIs(t, err, context.Canceled)
		assert.Less(t, time.Since(start), time.Second)
	})
}
