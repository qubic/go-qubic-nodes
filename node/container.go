package node

import (
	"cmp"
	"github.com/pkg/errors"
	"log"
	"slices"
	"sync"
	"time"
)

type Container struct {
	Addresses           []string
	ConfiguredAddresses []string
	Port                string
	TickErrorThreshold  uint32
	ReliableTickRange   uint32
	OnlineNodes         []*Node
	MaxTick             uint32
	LastUpdate          int64
	ReliableNodes       []*Node
	MostReliableNode    *Node
	mutexLock           sync.RWMutex
	connectionTimeout   time.Duration
	usePublicPeers      bool
}

type ContainerResponse struct {
	MaxTick          uint32
	LastUpdate       int64
	ReliableNodes    []*Node
	MostReliableNode *Node
}

func NewNodeContainer(addresses []string, port string, tickErrorThreshold, reliableTickRange uint32, connectionTimeout time.Duration, usePublicPeers bool) (*Container, error) {

	container := Container{
		ConfiguredAddresses: addresses,
		Addresses:           addresses,
		Port:                port,
		TickErrorThreshold:  tickErrorThreshold,
		ReliableTickRange:   reliableTickRange,
		connectionTimeout:   connectionTimeout,
		usePublicPeers:      usePublicPeers,
	}
	err := container.Update()
	if err != nil {
		return nil, errors.Wrap(err, "updating container after initialization")
	}

	return &container, nil
}

func (c *Container) Update() error {

	log.Printf("<==========REFRESH==========>\n")
	log.Printf("Refreshing nodes...\n")

	onlineNodes := fetchOnlineNodes(c.Addresses, c.Port, c.connectionTimeout)
	if c.usePublicPeers {
		lookForPublicPeers(onlineNodes, &c.Addresses, c.Port, c.connectionTimeout)
	}
	maxTick := calculateMaxTick(onlineNodes, c.TickErrorThreshold)

	reliableNodes, mostReliableNode := getReliableNodes(onlineNodes, maxTick, maxTick-c.ReliableTickRange)

	c.Set(onlineNodes, maxTick, time.Now().UTC().Unix(), reliableNodes, mostReliableNode)

	log.Printf("Node count: %d\n", c.GetNumberOfKnownNodes())
	log.Printf("Max tick: %d\n", maxTick)
	log.Printf("Reliable nodes: %d / %d online\n", len(reliableNodes), len(onlineNodes))
	if mostReliableNode != nil {
		log.Printf("Most reliable node: %s\n", mostReliableNode.Address)
	}

	return nil
}

func (c *Container) Set(OnlineNodes []*Node, MaxTick uint32, LastUpdate int64, ReliableNodes []*Node, MostReliableNode *Node) {
	c.mutexLock.Lock()
	defer c.mutexLock.Unlock()

	c.OnlineNodes = OnlineNodes
	c.MaxTick = MaxTick
	c.LastUpdate = LastUpdate
	c.ReliableNodes = ReliableNodes
	c.MostReliableNode = MostReliableNode
}

func (c *Container) GetResponse() ContainerResponse {
	c.mutexLock.RLock()
	defer c.mutexLock.RUnlock()

	return ContainerResponse{
		MaxTick:          c.MaxTick,
		LastUpdate:       c.LastUpdate,
		ReliableNodes:    c.ReliableNodes,
		MostReliableNode: c.MostReliableNode,
	}
}

func (c *Container) GetReliableNodesWithMinimumTick(tick uint32) []*Node {
	c.mutexLock.RLock()
	defer c.mutexLock.RUnlock()

	reliableNodesAtMinimumTick := make([]*Node, 0, len(c.ReliableNodes))
	for _, node := range c.ReliableNodes {
		if node.LastTick >= tick {
			reliableNodesAtMinimumTick = append(reliableNodesAtMinimumTick, node)
		}
	}

	return reliableNodesAtMinimumTick
}

func (c *Container) GetNumberOfConfiguredNodes() int {
	return len(c.ConfiguredAddresses)
}

func (c *Container) GetNumberOfKnownNodes() int {
	return len(c.Addresses)
}

func fetchOnlineNodes(addresses []string, port string, connectionTimeout time.Duration) []*Node {

	var waitGroup sync.WaitGroup

	nodesChannel := make(chan *Node, len(addresses))
	for _, address := range addresses {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			node, err := NewNode(address, port, connectionTimeout)
			if err != nil {
				log.Printf("Failed to create node: %v.\n", err)
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

func calculateMaxTick(nodes []*Node, threshold uint32) uint32 {
	slices.SortFunc(nodes, func(a, b *Node) int {
		return cmp.Compare(a.LastTick, b.LastTick)
	})

	arrayLength := len(nodes)

	if arrayLength == 0 {
		return 0
	}

	return nodes[arrayLength-1].LastTick
}

func getReliableNodes(onlineNodes []*Node, maximum, minimum uint32) ([]*Node, *Node) {

	reliableNodes := make([]*Node, 0, len(onlineNodes))

	var mostReliableNode *Node

	for _, node := range onlineNodes {

		if node.LastTick >= minimum && node.LastTick <= maximum {

			if node.LastTick == maximum {
				mostReliableNode = node
			}

			reliableNodes = append(reliableNodes, node)
		}
	}

	return reliableNodes, mostReliableNode
}

func lookForPublicPeers(nodes []*Node, addresses *[]string, port string, timeout time.Duration) {
	newPeers := make([]string, 0)
	for _, node := range nodes {
		peers := node.Peers
		for _, peer := range peers {
			if !slices.Contains(*addresses, peer) && !slices.Contains(newPeers, peer) {
				newPeers = append(newPeers, peer)
			}
		}
	}
	for _, peer := range newPeers {
		go appendNodeToAddresses(peer, port, timeout, addresses)
	}
}

func appendNodeToAddresses(host, port string, timeout time.Duration, addresses *[]string) {
	_, err := NewNode(host, port, timeout)
	if err == nil {
		log.Printf("Add new peer: [%s]\n", host)
		*addresses = append(*addresses, host)
	} // else do not use node
}
