package node

import (
	"log"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxNewPeersPerUpdate = 50

type UpdatedPeerList struct {
	originalPeers []string
	newPeers      []string
	excludedPeers []string
	mutex         sync.Mutex
}

func (pl *UpdatedPeerList) contains(host string) bool {
	return slices.Contains(pl.originalPeers, host) || slices.Contains(pl.newPeers, host)
}

func (pl *UpdatedPeerList) isAcceptedHost(host string) bool {
	return !slices.Contains(pl.excludedPeers, host)
}

func (pl *UpdatedPeerList) addIfNew(host string) bool {
	pl.mutex.Lock()
	defer pl.mutex.Unlock()
	if !pl.contains(host) {
		pl.newPeers = append(pl.newPeers, host)
		return true
	} else {
		return false
	}
}

type PeerDiscovery interface {
	FindNewPeers(currentNodes []*Node, currentAddresses []string) []*Node
	CleanupPeers(currentNodes []*Node, currentAddresses []string) []string
}

type PublicPeerDiscovery struct {
	createNodeFunction CreateNode
	excludedPeers      []string
	cleanInterval      time.Duration
	latestCleanup      time.Time
	lock               sync.Locker
}

type NoPeerDiscovery struct{}

func (npd NoPeerDiscovery) FindNewPeers(_ []*Node, _ []string) []*Node {
	return []*Node{}
}

func (npd NoPeerDiscovery) CleanupPeers(_ []*Node, _ []string) []string {
	return []string{}
}

func NewPublicPeerDiscovery(port string, connectionTimeout time.Duration, excludedPeers []string, cleanInterval time.Duration) *PublicPeerDiscovery {
	createNodeFunc := func(host string) (*Node, error) {
		return NewNode(host, port, connectionTimeout)
	}
	return newPublicPeerDiscovery(createNodeFunc, excludedPeers, cleanInterval)
}

func newPublicPeerDiscovery(createNodeFunc CreateNode, excludedPeers []string, cleanInterval time.Duration) *PublicPeerDiscovery {
	// trim host names
	var trimmed []string
	for _, peer := range excludedPeers {
		trimmed = append(trimmed, strings.TrimSpace(peer))
	}
	return &PublicPeerDiscovery{
		createNodeFunction: createNodeFunc,
		excludedPeers:      trimmed,
		cleanInterval:      cleanInterval,
		latestCleanup:      time.Now(),
		lock:               &sync.Mutex{},
	}
}

func (ppd PublicPeerDiscovery) CleanupPeers(nodes []*Node, addresses []string) []string {
	var unhealthyPeers []string
	if ppd.latestCleanup.Add(ppd.cleanInterval).Before(time.Now()) {
		for _, address := range addresses {
			if !slices.ContainsFunc(nodes, func(node *Node) bool { return node.Address == address }) {
				log.Printf("Unhealthy peer: [%s].", address)
				unhealthyPeers = append(unhealthyPeers, address)
			}
		}
	}
	return unhealthyPeers
}

func (ppd PublicPeerDiscovery) FindNewPeers(nodes []*Node, addresses []string) []*Node {
	peerCopy := make([]string, len(addresses))
	copy(peerCopy, addresses) // might get changed
	peers := &UpdatedPeerList{
		originalPeers: peerCopy,
		excludedPeers: ppd.excludedPeers,
		newPeers:      []string{},
	}

	var waitGroup sync.WaitGroup
	nodesChannel := make(chan *Node, maxNewPeersPerUpdate)
	for _, node := range nodes {
		ppd.lookupPeers(node.Peers, peers, nodesChannel, &waitGroup)
	}
	waitGroup.Wait()
	close(nodesChannel)

	var newNodes []*Node
	for node := range nodesChannel {
		if peers.isAcceptedHost(node.Address) {
			newNodes = append(newNodes, node)
		}
	}
	return newNodes
}

// recursive
func (ppd PublicPeerDiscovery) lookupPeers(hosts []string, peers *UpdatedPeerList, channel chan *Node, waitGroup *sync.WaitGroup) {
	for _, host := range hosts {
		// abort if channel is filled with next peer
		if len(channel) < maxNewPeersPerUpdate-2 && peers.addIfNew(host) {
			waitGroup.Add(1)
			go ppd.lookupPeer(host, peers, channel, waitGroup)
		}
	}
}

// recursive
func (ppd PublicPeerDiscovery) lookupPeer(host string, peers *UpdatedPeerList, channel chan *Node, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	node, err := ppd.createNodeFunction(host)
	if err == nil {
		channel <- node
		ppd.lookupPeers(node.Peers, peers, channel, waitGroup)
	}
}
