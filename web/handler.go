package web

import (
	"encoding/json"
	"github.com/qubic/go-node-connector/types"
	"github.com/qubic/go-qubic-nodes/node"

	"log"
	"net/http"
)

type RequestHandler struct {
	Container *node.Container
}

type statusResponse struct {
	MaxTick          uint32         `json:"max_tick"`
	LastUpdate       int64          `json:"last_update"`
	ReliableNodes    []nodeResponse `json:"reliable_nodes"`
	MostReliableNode nodeResponse   `json:"most_reliable_node"`
}

type nodeResponse struct {
	Address    string            `json:"address"`
	Peers      types.PublicPeers `json:"peers"`
	LastTick   uint32            `json:"last_tick"`
	LastUpdate int64             `json:"last_update"`
}

type maxTickResponse struct {
	MaxTick uint32 `json:"max_tick"`
}

func (h *RequestHandler) HandleStatus(writer http.ResponseWriter, request *http.Request) {

	containerResponse := h.Container.GetResponse()

	var reliableNodes []nodeResponse
	for _, reliableNode := range containerResponse.ReliableNodes {
		r := nodeResponse{
			Address:    reliableNode.Address,
			Peers:      reliableNode.Peers,
			LastTick:   reliableNode.LastTick,
			LastUpdate: reliableNode.LastUpdate,
		}
		reliableNodes = append(reliableNodes, r)
	}

	mostReliable := containerResponse.MostReliableNode

	mostReliableResponse := nodeResponse{
		Address:    mostReliable.Address,
		Peers:      mostReliable.Peers,
		LastTick:   mostReliable.LastTick,
		LastUpdate: mostReliable.LastUpdate,
	}

	response := statusResponse{
		MaxTick:          containerResponse.MaxTick,
		LastUpdate:       containerResponse.LastUpdate,
		ReliableNodes:    reliableNodes,
		MostReliableNode: mostReliableResponse,
	}

	data, err := json.Marshal(response)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		//TODO: Handle?
		writer.Write([]byte(err.Error()))
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write(data)
	if err != nil {
		log.Printf("Failed to write response for status request. Err: %v\n", err)
	}

}

func (h *RequestHandler) HandleMaxTick(writer http.ResponseWriter, request *http.Request) {

	maxTick := h.Container.GetResponse().MaxTick

	response := maxTickResponse{
		maxTick,
	}

	data, err := json.Marshal(response)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		//TODO: Handle?
		writer.Write([]byte(err.Error()))
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write(data)
	if err != nil {
		log.Printf("Failed to write response for max-tick request. Err: %v\n", err)
	}
}
