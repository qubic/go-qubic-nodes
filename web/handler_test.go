package web

import (
	"github.com/qubic/go-qubic-nodes/node"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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

	var container = node.Container{

		Addresses:          []string{node1.Address, node2.Address},
		Port:               "12345",
		TickErrorThreshold: 3,
		ReliableTickRange:  4,
		OnlineNodes:        nil,
		MaxTick:            123,
		LastUpdate:         1500000000,
		ReliableNodes:      []*node.Node{&node1},
		MostReliableNode:   &node1,
	}

	handler := RequestHandler{
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
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "Unexpected content type header")
	assert.Equal(t, 200, resp.StatusCode, "Unexpected http status")
	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.JSONEq(t, expectedResponse, string(data))
}

func makeStatusCall(handler RequestHandler) *http.Response {
	rec := httptest.NewRecorder()
	handler.HandleStatus(rec, nil)
	resp := rec.Result()
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	return resp
}
