package node

import (
	"log"
	"slices"
	"sync"
	"time"
)

type Map map[string]*Node

type Container struct {
	Addresses          []string `json:"-"`
	TickErrorThreshold uint32   `json:"-"`
	ReliableTickRange  uint32   `json:"-"`
	OnlineNodes        *Map     `json:"-"`
	MaxTick            uint32   `json:"max_tick"`
	LastUpdate         int64    `json:"last_update"`
	ReliableNodes      []*Node  `json:"reliable_nodes"`
	MostReliableNode   *Node    `json:"most_reliable_node"`
	mutexLock          sync.RWMutex
	connectionTimeout  time.Duration
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

func (container *Container) Update() {

	container.mutexLock.RLock()
	defer container.mutexLock.RUnlock()

	//TODO: Maybe use node.Update() instead of creating new nodes every time...
	onlineNodes, tickList := fetchOnlineNodes(container.Addresses, container.connectionTimeout)
	container.OnlineNodes = onlineNodes
	slices.Sort(tickList)

	maxTick := calculateMaxTick(tickList, container.TickErrorThreshold)
	container.MaxTick = maxTick

	container.LastUpdate = time.Now().UTC().Unix()

	reliableNodes, mostReliableNode := getReliableNodes(onlineNodes, maxTick, maxTick-container.ReliableTickRange)
	container.ReliableNodes = reliableNodes
	container.MostReliableNode = mostReliableNode

}

func fetchOnlineNodes(addresses []string, connectionTimeout time.Duration) (*Map, []uint32) {

	onlineNodes := Map{}
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
