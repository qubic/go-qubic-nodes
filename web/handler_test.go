package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/qubic/go-qubic-nodes/node"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandler_whenStatus_thenReturnNumberOfConfiguredNodes(t *testing.T) {

	var node1 = node.Node{
		Address:           "1.2.3.4",
		Port:              "12345",
		Peers:             []string{"2.3.4.5", "3.4.5.6"},
		LastTick:          123,
		LastUpdate:        1500000000,
		LastUpdateSuccess: true,
	}

	var node2 = node.Node{
		Address:           "2.3.4.5",
		Port:              "12345",
		Peers:             []string{"3.4.5.6", "1.2.3.4"},
		LastTick:          122,
		LastUpdate:        1500000000,
		LastUpdateSuccess: true,
	}

	peerManager := node.NewPeerManager([]string{node1.Address, node2.Address}, &node.NoPeerDiscovery{}, "12345", time.Second)

	var container = node.Container{
		PeerManager:        peerManager,
		TickErrorThreshold: 3,
		ReliableTickRange:  4,
		OnlineNodes:        nil,
		MaxTick:            123,
		LastUpdate:         1500000000,
		ReliableNodes:      []*node.Node{&node1},
		MostReliableNode:   &node1,
	}

	handler := PeersHandler{
		Container: &container,
	}

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

func TestPeersHandler_GetReliableNodesWithMinimumTick(t *testing.T) {
	testData := []struct {
		name                  string
		nodes                 []*node.Node
		minimumTick           uint32
		expectedReliableNodes []*node.Node
	}{
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_1_node_below_minimum",
			nodes: []*node.Node{
				{
					LastTick: 1992,
				},
			},
			minimumTick:           1993,
			expectedReliableNodes: []*node.Node{},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_2_node_below_minimum",
			nodes: []*node.Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
			},
			minimumTick:           1993,
			expectedReliableNodes: []*node.Node{},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_1_node_above_minimum",
			nodes: []*node.Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
				{
					LastTick: 1994,
				},
			},
			minimumTick: 1993,
			expectedReliableNodes: []*node.Node{
				{
					LastTick: 1994,
				},
			},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_1_node_equal_minimum",
			nodes: []*node.Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
				{
					LastTick: 1993,
				},
			},
			minimumTick: 1993,
			expectedReliableNodes: []*node.Node{
				{
					LastTick: 1993,
				},
			},
		},
		{
			name: "TestContainer_GetReliableNodesWithMinimumTick_2_node_equal_and_above_minimum",
			nodes: []*node.Node{
				{
					LastTick: 1992,
				},
				{
					LastTick: 1991,
				},
				{
					LastTick: 1993,
				},
				{
					LastTick: 1994,
				},
			},
			minimumTick: 1993,
			expectedReliableNodes: []*node.Node{
				{
					LastTick: 1993,
				},
				{
					LastTick: 1994,
				},
			},
		},
	}

	testFunc := func(t *testing.T, minimumTick uint32, nodes []*node.Node, expectedReliablePeers []*node.Node) func(t *testing.T) {

		return func(t *testing.T) {
			container := &node.Container{}
			container.ReliableNodes = nodes

			handler := PeersHandler{Container: container}
			resp, err := makeGetReliableNodesWithMinimumTickCall(handler, minimumTick)
			require.NoError(t, err, "making reliable nodes call")
			expectedResponse := reliablePeersAtMinimumTickResponse{
				RequestedMinimumTick: minimumTick,
				ReliableNodes:        expectedReliablePeers,
			}

			diff := cmp.Diff(expectedResponse, resp)
			require.Empty(t, diff)
		}
	}

	for _, test := range testData {
		t.Run(test.name, testFunc(t, test.minimumTick, test.nodes, test.expectedReliableNodes))
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
		return reliablePeersAtMinimumTickResponse{}, errors.Wrap(err, "decoding response body")
	}

	return respBody, nil
}
