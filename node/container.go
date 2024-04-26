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

func NewNodeContainer(addresses []string, tickErrorThreshold, reliableTickRange uint32, connectionTimeout time.Duration) (*Container, error) {

	container := Container{
		Addresses:          addresses,
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

	onlineNodes := fetchOnlineNodes(c.Addresses, c.connectionTimeout)
	slices.SortFunc(onlineNodes, func(a, b *Node) int {
		return cmp.Compare(a.LastTick, b.LastTick)
	})
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

func fetchOnlineNodes(addresses []string, connectionTimeout time.Duration) []*Node {

	var onlineNodes []*Node

	for _, address := range addresses {
		node, err := NewNode(address, connectionTimeout)
		if err != nil {
			log.Printf("Failed to create node: %v\n", err)
			continue
		}

		onlineNodes = append(onlineNodes, node)
	}

	return onlineNodes

}

func calculateMaxTick(nodes []*Node, threshold uint32) uint32 {

	arrayLength := len(nodes)

	if arrayLength == 0 {
		return 0
	}

	if arrayLength < 2 {
		return nodes[0].LastTick
	}

	maxTick := nodes[len(nodes)-1].LastTick
	maxTick2 := nodes[len(nodes)-2].LastTick

	if maxTick2 != 0 && (maxTick-maxTick2) >= threshold {
		return maxTick2
	}
	return maxTick
}

func getReliableNodes(onlineNodes []*Node, maximum, minimum uint32) ([]*Node, *Node) {

	var reliableNodes []*Node

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
