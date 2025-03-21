package node

import (
	"log"
	"slices"
	"strings"
	"sync"
	"time"
)

type PeerManager struct {
	configuredPeers    []string
	currentPeers       []string
	peerDiscovery      PeerDiscovery
	createNodeFunction CreateNode
}

type CreateNode func(host string) (*Node, error)

func NewPeerManager(addresses []string, peerDiscovery PeerDiscovery, port string, connectionTimeout time.Duration) *PeerManager {
	crateNodeFunc := func(host string) (*Node, error) {
		return NewNode(host, port, connectionTimeout)
	}
	return newPeerManagerWithCreateNodeFunction(addresses, peerDiscovery, crateNodeFunc)
}

// mainly for testing to inject custom node creation code
func newPeerManagerWithCreateNodeFunction(addresses []string, peerDiscovery PeerDiscovery, createNodeFunction CreateNode) *PeerManager {
	// trim host names
	var trimmed []string
	for _, peer := range addresses {
		trimmed = append(trimmed, strings.TrimSpace(peer))
	}
	configuredPeers := make([]string, len(trimmed))
	copy(configuredPeers, trimmed) // assure that they are not changed by changing current peers
	peerManager := PeerManager{
		configuredPeers:    configuredPeers,
		currentPeers:       trimmed,
		createNodeFunction: createNodeFunction,
		peerDiscovery:      peerDiscovery,
	}
	return &peerManager
}

func (pm *PeerManager) UpdateNodes() []*Node {
	onlineNodes := pm.fetchOnlineNodes()
	go pm.updatePeers(onlineNodes)
	return onlineNodes
}

func (pm *PeerManager) fetchOnlineNodes() []*Node {

	var waitGroup sync.WaitGroup

	nodesChannel := make(chan *Node, len(pm.currentPeers))
	for _, address := range pm.currentPeers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			node, err := pm.createNodeFunction(address)
			if err != nil {
				log.Printf("Failed to create node: %v.", err)
				nodesChannel <- nil
				return
			}

			nodesChannel <- node
		}()
	}

	waitGroup.Wait()
	close(nodesChannel)

	var onlineNodes []*Node
	for node := range nodesChannel {
		if node != nil {
			onlineNodes = append(onlineNodes, node)
		}
	}
	return onlineNodes
}

func (pm *PeerManager) GetNumberOfConfiguredNodes() int {
	return len(pm.configuredPeers)
}

func (pm *PeerManager) GetNumberOfKnownNodes() int {
	return len(pm.currentPeers)
}

func (pm *PeerManager) updatePeers(nodes []*Node) {

	unhealthyPeers := pm.peerDiscovery.CleanupPeers(nodes, pm.currentPeers)
	for _, host := range unhealthyPeers {
		if !slices.Contains(pm.configuredPeers, host) { // don't remove configured nodes
			// delete unhealthy peer from current peer list
			log.Printf("Remove peer: [%s].", host)
			pm.currentPeers = slices.DeleteFunc(pm.currentPeers, func(currentHost string) bool {
				return strings.TrimSpace(currentHost) == host
			})
		}
	}

	newPeers := pm.peerDiscovery.FindNewPeers(nodes, pm.currentPeers)
	for _, newPeer := range newPeers {
		if !slices.Contains(pm.currentPeers, newPeer.Address) {
			log.Printf("Add peer: [%s].", newPeer.Address)
			pm.currentPeers = append(pm.currentPeers, newPeer.Address)
		}
	}
}
