package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/qubic/go-node-connector/v2/types"
	"github.com/qubic/go-qubic-nodes/metrics"
	"github.com/qubic/go-qubic-nodes/peer"
	"github.com/stretchr/testify/require"
)

// stubPeer is a peer.Prober serving fixed values. The handler only ever reads the
// last known state, so the probe itself is never exercised here.
type stubPeer struct {
	address string
	port    string
	tick    uint32
	peers   types.PublicPeers
}

func (s *stubPeer) GetPeerInfo(context.Context, time.Duration) (peer.Info, error) {
	return peer.Info{}, nil
}

func (s *stubPeer) GetAddress() string                   { return s.address }
func (s *stubPeer) GetPort() string                      { return s.port }
func (s *stubPeer) GetLastKnownTick() uint32             { return s.tick }
func (s *stubPeer) GetLastKnownPeers() types.PublicPeers { return s.peers }

// newTestHandler builds a handler over a Manager pinned to a known state, so the
// responses under test depend only on that state and not on any network activity.
func newTestHandler(t *testing.T, seedPeers []string, maxTick uint32, lastUpdate int64, peers ...*stubPeer) PeersHandler {
	t.Helper()

	reliablePeers := make(map[string]peer.Prober, len(peers))
	for _, p := range peers {
		reliablePeers[p.address] = p
	}

	manager := peer.NewPeerManager(
		peer.ManagerConfig{
			SeedPeers: seedPeers,
			MaxPeers:  len(seedPeers),
		},
		metrics.NewNodesServiceMetrics(prometheus.NewRegistry(), "test"),
		peer.WithInitialPeers(reliablePeers),
		peer.WithInitialStatus(maxTick, lastUpdate),
	)

	return PeersHandler{PeerManager: manager}
}

func TestHandler_whenStatus_thenReturnNumberOfConfiguredNodes(t *testing.T) {
	node1 := &stubPeer{
		address: "1.2.3.4",
		port:    "12345",
		tick:    123,
		peers:   []string{"2.3.4.5", "3.4.5.6"},
	}

	handler := newTestHandler(t, []string{"1.2.3.4", "2.3.4.5"}, 123, 1500000000, node1)

	expectedResponse := `{
		"max_tick": 123,
		"last_update": 1500000000,
		"number_of_configured_nodes": 2,
		"reliable_nodes": [
			{
			  "address": "1.2.3.4",
			  "port": "12345",
			  "peers": [
				"2.3.4.5",
				"3.4.5.6"
			  ],
			  "last_tick": 123,
			  "last_update": 1500000000
			}
		],
		"most_reliable_node": {
			"address": "1.2.3.4",
			"port": "12345",
			"peers": [
				"2.3.4.5",
				"3.4.5.6"
			],
			"last_tick": 123,
			"last_update": 1500000000
		}
	}`

	resp := makeStatusCall(handler)
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"), "Unexpected content type header")
	require.Equal(t, 200, resp.StatusCode, "Unexpected http status")
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.JSONEq(t, expectedResponse, string(data))
}

func TestHandler_whenStatusWithoutReliablePeers_thenServiceUnavailable(t *testing.T) {
	handler := newTestHandler(t, []string{"1.2.3.4"}, 0, 1500000000)

	resp := makeStatusCall(handler)

	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestPeersHandler_GetReliableNodesWithMinimumTick(t *testing.T) {
	testData := []struct {
		name        string
		peers       []*stubPeer
		minimumTick uint32
		wantTicks   []uint32
	}{
		{
			name:        "1_node_below_minimum",
			peers:       []*stubPeer{{address: "1.1.1.1", tick: 1992}},
			minimumTick: 1993,
			wantTicks:   []uint32{},
		},
		{
			name:        "2_nodes_below_minimum",
			peers:       []*stubPeer{{address: "1.1.1.1", tick: 1992}, {address: "2.2.2.2", tick: 1991}},
			minimumTick: 1993,
			wantTicks:   []uint32{},
		},
		{
			name: "1_node_above_minimum",
			peers: []*stubPeer{
				{address: "1.1.1.1", tick: 1992},
				{address: "2.2.2.2", tick: 1991},
				{address: "3.3.3.3", tick: 1994},
			},
			minimumTick: 1993,
			wantTicks:   []uint32{1994},
		},
		{
			name: "1_node_equal_minimum",
			peers: []*stubPeer{
				{address: "1.1.1.1", tick: 1992},
				{address: "2.2.2.2", tick: 1991},
				{address: "3.3.3.3", tick: 1993},
			},
			minimumTick: 1993,
			wantTicks:   []uint32{1993},
		},
		{
			name: "2_nodes_equal_and_above_minimum",
			peers: []*stubPeer{
				{address: "1.1.1.1", tick: 1992},
				{address: "2.2.2.2", tick: 1991},
				{address: "3.3.3.3", tick: 1993},
				{address: "4.4.4.4", tick: 1994},
			},
			minimumTick: 1993,
			wantTicks:   []uint32{1993, 1994},
		},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			handler := newTestHandler(t, []string{"1.1.1.1"}, 1994, 1500000000, test.peers...)

			resp, err := makeGetReliableNodesWithMinimumTickCall(handler, test.minimumTick)
			require.NoError(t, err, "making reliable nodes call")

			require.Equal(t, test.minimumTick, resp.RequestedMinimumTick)
			// Length is asserted separately from contents: a response padded with
			// zero-valued entries would still contain every expected tick.
			require.Len(t, resp.ReliableNodes, len(test.wantTicks))

			gotTicks := make([]uint32, 0, len(resp.ReliableNodes))
			for _, n := range resp.ReliableNodes {
				gotTicks = append(gotTicks, n.LastTick)
			}
			require.ElementsMatch(t, test.wantTicks, gotTicks)
		})
	}
}

func makeStatusCall(handler PeersHandler) *http.Response {
	rec := httptest.NewRecorder()
	handler.HandleStatus(rec, nil)
	resp := rec.Result()
	defer resp.Body.Close()
	return resp
}

func makeGetReliableNodesWithMinimumTickCall(handler PeersHandler, minimumTick uint32) (reliablePeersAtMinimumTickResponse, error) {
	rec := httptest.NewRecorder()
	body := `{"minimum_tick": ` + fmt.Sprintf("%d", minimumTick) + `}`
	req := httptest.NewRequest("POST", "/reliable-nodes", bytes.NewBuffer([]byte(body)))
	defer req.Body.Close()

	handler.GetReliableNodesWithMinimumTick(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()

	var respBody reliablePeersAtMinimumTickResponse
	err := json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return reliablePeersAtMinimumTickResponse{}, fmt.Errorf("decoding response body: %w", err)
	}

	return respBody, nil
}
