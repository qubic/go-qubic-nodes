package web

import (
	"encoding/json"

	"github.com/qubic/go-node-connector/v2/types"
	"github.com/qubic/go-qubic-nodes/peer"

	"log"
	"net/http"
)

type PeersHandler struct {
	PeerManager *peer.Manager
}

type statusResponse struct {
	MaxTick                 uint32         `json:"max_tick"`
	LastUpdate              int64          `json:"last_update"`
	NumberOfConfiguredNodes int            `json:"number_of_configured_nodes"`
	ReliableNodes           []reliableNode `json:"reliable_nodes"`
	MostReliableNode        reliableNode   `json:"most_reliable_node"`
}

type reliableNode struct {
	Address    string            `json:"address"`
	Port       string            `json:"port"`
	Peers      types.PublicPeers `json:"peers"`
	LastTick   uint32            `json:"last_tick"`
	LastUpdate int64             `json:"last_update"`
}

type maxTickResponse struct {
	MaxTick uint32 `json:"max_tick"`
}

type reliablePeersAtMinimumTickResponse struct {
	RequestedMinimumTick uint32 `json:"requested_minimum_tick"`
	ReliableNodes        []node `json:"reliable_nodes"`
}

func (h *PeersHandler) HandleStatus(w http.ResponseWriter, _ *http.Request) {

	peerManagerStatus := h.PeerManager.GetStatus()

	if len(peerManagerStatus.ReliablePeers) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := w.Write([]byte("No online or reliable nodes found."))
		if err != nil {
			log.Printf("Failed to respond to request: %v\n", err)
		}
		return
	}

	var reliableNodes []reliableNode
	for _, relNode := range peerManagerStatus.ReliablePeers {
		r := reliableNode{
			Address:    relNode.GetAddress(),
			Port:       relNode.GetPort(),
			Peers:      relNode.GetLastKnownPeers(),
			LastTick:   relNode.GetLastKnownTick(),
			LastUpdate: peerManagerStatus.LastUpdate,
		}
		reliableNodes = append(reliableNodes, r)
	}

	mostReliable := peerManagerStatus.MostReliablePeer

	mostReliableResponse := reliableNode{
		Address:    mostReliable.GetAddress(),
		Port:       mostReliable.GetPort(),
		Peers:      mostReliable.GetLastKnownPeers(),
		LastTick:   mostReliable.GetLastKnownTick(),
		LastUpdate: peerManagerStatus.LastUpdate,
	}

	response := statusResponse{
		MaxTick:                 peerManagerStatus.MaxTick,
		LastUpdate:              peerManagerStatus.LastUpdate,
		NumberOfConfiguredNodes: h.PeerManager.GetPeerCap(),
		ReliableNodes:           reliableNodes,
		MostReliableNode:        mostReliableResponse,
	}

	data, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			log.Printf("Failed to respond to request: %v\n", err)
		}
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		log.Printf("Failed to write response for status request. Err: %v\n", err)
	}

}

func (h *PeersHandler) HandleMaxTick(writer http.ResponseWriter, _ *http.Request) {

	maxTick := h.PeerManager.GetStatus().MaxTick

	response := maxTickResponse{
		maxTick,
	}

	data, err := json.Marshal(response)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, err := writer.Write([]byte(err.Error()))
		if err != nil {
			log.Printf("Failed to respond to request: %v\n", err)
		}
		return
	}

	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write(data)
	if err != nil {
		log.Printf("Failed to write response for max-tick request. Err: %v\n", err)
	}
}

type node struct {
	Address           string
	Port              string
	Peers             types.PublicPeers
	LastTick          uint32
	LastUpdate        int64
	LastUpdateSuccess bool
}

func (h *PeersHandler) GetReliableNodesWithMinimumTick(w http.ResponseWriter, r *http.Request) {
	var mtr struct {
		MinimumTick uint32 `json:"minimum_tick"`
	}

	err := json.NewDecoder(r.Body).Decode(&mtr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			log.Printf("Failed to respond to request: %v\n", err)
		}
		return
	}

	reliablePeers, lastUpdate := h.PeerManager.GetReliablePeersWithMinimumTick(mtr.MinimumTick)
	reliableNodes := make([]node, 0, len(reliablePeers))
	for _, reliablePeer := range reliablePeers {
		reliableNodes = append(reliableNodes, node{
			Address:           reliablePeer.GetAddress(),
			Port:              reliablePeer.GetPort(),
			Peers:             reliablePeer.GetLastKnownPeers(),
			LastTick:          reliablePeer.GetLastKnownTick(),
			LastUpdate:        lastUpdate,
			LastUpdateSuccess: true,
		})
	}

	responseData := reliablePeersAtMinimumTickResponse{
		RequestedMinimumTick: mtr.MinimumTick,
		ReliableNodes:        reliableNodes,
	}

	w.Header().Add("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(responseData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			log.Printf("Failed to respond to request: %v\n", err)
		}
		return
	}
}
