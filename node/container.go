package node

import (
	"log"
	"slices"
	"sync"
	"time"
)

type Map map[string]*Node

type Container struct {
	Addresses          []string
	TickErrorThreshold uint32
	ReliableTickRange  uint32
	OnlineNodes        *Map
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

func NewNodeContainer(addresses []string, tickErrorThreshold, reliableTickRange uint32, connectionTimeout time.Duration) *Container {

	container := Container{
		Addresses:          addresses,
		TickErrorThreshold: tickErrorThreshold,
		ReliableTickRange:  reliableTickRange,
		connectionTimeout:  connectionTimeout,
	}
	container.Update()

	return &container
}

func (c *Container) Update() {

	//TODO: Maybe use node.Update() instead of creating new nodes every time...
	onlineNodes, tickList := fetchOnlineNodes(c.Addresses, c.connectionTimeout)
	slices.Sort(tickList)
	maxTick := calculateMaxTick(tickList, c.TickErrorThreshold)
	reliableNodes, mostReliableNode := getReliableNodes(onlineNodes, maxTick, maxTick-c.ReliableTickRange)

	c.Set(onlineNodes, maxTick, time.Now().UTC().Unix(), reliableNodes, mostReliableNode)
}

func (c *Container) Set(OnlineNodes *Map, MaxTick uint32, LastUpdate int64, ReliableNodes []*Node, MostReliableNode *Node) {
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

func fetchOnlineNodes(addresses []string, connectionTimeout time.Duration) (*Map, []uint32) {

	onlineNodes := make(Map)
	tickList := make([]uint32, 0)

	for _, address := range addresses {
		node, err := NewNode(address, connectionTimeout)
		if err != nil {
			log.Printf("Failed to create node: %v\n", err)
			continue
		}

		onlineNodes[address] = node
		tickList = append(tickList, node.LastTick)
	}

	return &onlineNodes, tickList

}

func calculateMaxTick(tickList []uint32, threshold uint32) uint32 {
	maxTick := tickList[len(tickList)-1]
	maxTick2 := tickList[len(tickList)-2]

	if maxTick2 != 0 && (maxTick-maxTick2) >= threshold {
		return maxTick2
	}
	return maxTick
}

func getReliableNodes(onlineNodes *Map, maximum, minimum uint32) ([]*Node, *Node) {

	var reliableNodes []*Node

	var mostReliableNode *Node

	for _, node := range *onlineNodes {

		if node.LastTick >= minimum && node.LastTick <= maximum {

			if node.LastTick == maximum {
				mostReliableNode = node
			}

			reliableNodes = append(reliableNodes, node)
		}
	}

	return reliableNodes, mostReliableNode
}
