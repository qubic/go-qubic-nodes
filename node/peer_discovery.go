package node

import (
	"log"
	"slices"
	"sync"
	"time"
)

const maxNewPeers = 100

type PeerDiscovery interface {
	UpdatePeers(currentNodes []*Node, currentAddresses []string) []*Node
}

type PublicPeerDiscovery struct {
	createNodeFunction CreateNode
	lock               sync.Locker
}

type PeerList struct {
	originalPeers []string
	newPeers      []string
	mutex         sync.Mutex
}

func (pl *PeerList) contains(host string) bool {
	return slices.Contains(pl.originalPeers, host) || slices.Contains(pl.newPeers, host)
}

func (pl *PeerList) AddIfNew(host string) bool {
	pl.mutex.Lock()
	defer pl.mutex.Unlock()
	if !pl.contains(host) {
		pl.newPeers = append(pl.newPeers, host)
		return true
	} else {
		return false
	}
}

type NoPeerDiscovery struct{}

func (npd NoPeerDiscovery) UpdatePeers(_ []*Node, _ []string) []*Node {
	return []*Node{}
}

func NewPublicPeerDiscovery(port string, connectionTimeout time.Duration) *PublicPeerDiscovery {
	createNodeFunc := func(host string) (*Node, error) {
		return NewNode(host, port, connectionTimeout)
	}
	return newPublicPeerDiscovery(createNodeFunc)
}

func newPublicPeerDiscovery(createNodeFunc CreateNode) *PublicPeerDiscovery {
	return &PublicPeerDiscovery{
		createNodeFunction: createNodeFunc,
		lock:               &sync.Mutex{},
	}
}

func (ppd PublicPeerDiscovery) UpdatePeers(nodes []*Node, addresses []string) []*Node {
	peerList := &PeerList{
		originalPeers: addresses,
		newPeers:      make([]string, 0),
	}

	var waitGroup sync.WaitGroup
	nodesChannel := make(chan *Node, maxNewPeers) // TODO move constant
	for _, node := range nodes {
		ppd.lookupPeers(node.Peers, peerList, nodesChannel, &waitGroup)
	}
	waitGroup.Wait()
	close(nodesChannel)

	var newNodes []*Node
	for node := range nodesChannel {
		newNodes = append(newNodes, node)
	}
	if len(newNodes) > 0 {
		log.Printf("Found [%d] potential new peer(s).", len(newNodes))
	}
	return newNodes
}

// recursive
func (ppd PublicPeerDiscovery) lookupPeers(hosts []string, peers *PeerList, channel chan *Node, waitGroup *sync.WaitGroup) {

	for _, host := range hosts {
		// abort if channel is filled with next peer
		if len(channel) < maxNewPeers-2 && peers.AddIfNew(host) {
			waitGroup.Add(1)
			go ppd.lookupPeer(host, peers, channel, waitGroup)
		}
	}
}

// recursive
func (ppd PublicPeerDiscovery) lookupPeer(host string, peers *PeerList, channel chan *Node, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	node, err := ppd.createNodeFunction(host)
	if err == nil {
		channel <- node
		ppd.lookupPeers(node.Peers, peers, channel, waitGroup)
	} else {
		log.Printf("Failed to create node: %v\n", err)
	}
}
