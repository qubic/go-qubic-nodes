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
	Addresses          []string
	Port               string
	TickErrorThreshold uint32
	ReliableTickRange  uint32
	OnlineNodes        []*Node
	MaxTick            uint32
	LastUpdate         int64
	ReliableNodes      []*Node
	MostReliableNode   *Node
	mutexLock          sync.RWMutex
	connectionTimeout  time.Duration
}

type ContainerResponse struct {
	MaxTick          uint32
	LastUpdate       int64
	ReliableNodes    []*Node
	MostReliableNode *Node
}

func NewNodeContainer(addresses []string, port string, tickErrorThreshold, reliableTickRange uint32, connectionTimeout time.Duration) (*Container, error) {

	container := Container{
		Addresses:          addresses,
		Port:               port,
		TickErrorThreshold: tickErrorThreshold,
		ReliableTickRange:  reliableTickRange,
		connectionTimeout:  connectionTimeout,
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
	maxTick := calculateMaxTick(onlineNodes, c.TickErrorThreshold)

	reliableNodes, mostReliableNode := getReliableNodes(onlineNodes, maxTick, maxTick-c.ReliableTickRange)

	c.Set(onlineNodes, maxTick, time.Now().UTC().Unix(), reliableNodes, mostReliableNode)

	log.Printf("Ip count: %d\n", len(c.Addresses))
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

func fetchOnlineNodes(addresses []string, port string, connectionTimeout time.Duration) []*Node {

	var waitGroup sync.WaitGroup

	nodesChannel := make(chan *Node, len(addresses))

	for _, address := range addresses {
		waitGroup.Add(1)

		go func(nodesChannel chan<- *Node) {
			defer waitGroup.Done()

			log.Printf("Attempting to create node %s:%s\n", address, port)
			now := time.Now()
			node, err := NewNode(address, port, connectionTimeout)
			if err != nil {
				elapsed := time.Since(now)
				log.Printf("Failed to create node: %v. Took %fs\n", err, elapsed.Seconds())
				nodesChannel <- nil
				return
			}

			elapsed := time.Since(now)
			log.Printf("Got online node %s:%s. Took %fs\n", address, port, elapsed.Seconds())
			nodesChannel <- node
		}(nodesChannel)
	}

	var onlineNodes []*Node
	for range len(addresses) {
		node := <-nodesChannel
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
