package node

import (
	"github.com/pkg/errors"
	"github.com/qubic/go-node-connector/types"
	"log"
	"time"
)

const nodePort = "21841"

type Node struct {
	Address           string            `json:"address"`
	Peers             types.PublicPeers `json:"peers"`
	LastTick          uint32            `json:"last_tick"`
	LastUpdate        int64             `json:"last_update"`
	LastUpdateSuccess bool              `json:"last_update_success"`
}

func NewNode(ip string, connectionTimeout time.Duration) (*Node, error) {

	client, ctx, cancel, err := NewNodeConnection(ip, connectionTimeout)
	defer cancel()
	if err != nil {
		return nil, errors.Wrap(err, "creating node connection")
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting tick info from node")
	}

	log.Printf("Found online node: %s - %d\n", ip, tickInfo.Tick)

	var node = Node{
		ip,
		client.Peers,
		tickInfo.Tick,
		time.Now().UTC().Unix(),
		true,
	}

	return &node, nil
}

func (node *Node) Update(connectionTimeout time.Duration) error {
	client, ctx, cancel, err := NewNodeConnection(node.Address, connectionTimeout)
	defer cancel()
	if err != nil {
		node.LastUpdateSuccess = false
		return errors.Wrap(err, "creating node connection")
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		node.LastUpdateSuccess = false
		return errors.Wrap(err, "getting tick info from node")
	}

	node.Peers = client.Peers
	node.LastTick = tickInfo.Tick
	node.LastUpdate = time.Now().UTC().Unix()
	node.LastUpdateSuccess = true

	return nil

}
