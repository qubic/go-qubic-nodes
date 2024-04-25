package web

import (
	"encoding/json"
	"github.com/qubic/go-qubic-nodes/node"

	"log"
	"net/http"
)

type RequestHandler struct {
	Container *node.Container
}

type statusResponse struct {
	MaxTick          uint32       `json:"max_tick"`
	LastUpdate       int64        `json:"last_update"`
	ReliableNodes    []*node.Node `json:"reliable_nodes"`
	MostReliableNode *node.Node   `json:"most_reliable_node"`
}

func (h *RequestHandler) HandleStatus(writer http.ResponseWriter, request *http.Request) {

	c := h.Container.GetResponse()

	response := statusResponse{
		MaxTick:          c.MaxTick,
		LastUpdate:       c.LastUpdate,
		ReliableNodes:    c.ReliableNodes,
		MostReliableNode: c.MostReliableNode,
	}

	data, err := json.Marshal(response)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
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
