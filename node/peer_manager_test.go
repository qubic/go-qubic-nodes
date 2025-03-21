package node

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
	"time"
)

var testTime = time.Now()

func createTestNodes(host string) (*Node, error) {
	if host == "6.6.6.6" {
		return nil, errors.New("error creating test node")
	} else {
		return createTestNode(host), nil
	}
}

func TestPeerManager_UpdateNodes(t *testing.T) {
	peerDiscovery := NoPeerDiscovery{}
	peerManager := newPeerManagerWithCreateNodeFunction([]string{"1.2.3.4", "6.6.6.6", "2.3.4.5"}, peerDiscovery, createTestNodes)

	nodes := peerManager.UpdateNodes()

	assert.Len(t, nodes, 2)
	assert.Contains(t, nodes, createTestNode("1.2.3.4"))
	assert.Contains(t, nodes, createTestNode("2.3.4.5"))
}

func createTestNode(host string) *Node {
	return createTestNodeWithPeers(host, []string{})
}

func createTestNodeWithPeers(host string, peers []string) *Node {
	log.Printf("Creating test node [%s]", host)
	return &Node{
		Address:           host,
		Port:              "12345",
		Peers:             peers,
		LastTick:          42,
		LastUpdate:        testTime.UTC().Unix(),
		LastUpdateSuccess: true,
	}
}
