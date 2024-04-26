package node

import (
	"context"
	"github.com/pkg/errors"
	qubic "github.com/qubic/go-node-connector"
	"github.com/qubic/go-node-connector/types"
	"log"
	"time"
)

const nodePort = "21841"

type Node struct {
	Address           string
	Peers             types.PublicPeers
	LastTick          uint32
	LastUpdate        int64
	LastUpdateSuccess bool
}

func NewNode(ip string, connectionTimeout time.Duration) (*Node, error) {

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	client, err := qubic.NewClient(ctx, ip, nodePort)
	if err != nil {
		return nil, errors.Wrap(err, "creating node connection")
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting tick info from node")
	}

	log.Printf("Found online node: %s - %d\n", ip, tickInfo.Tick)

	node := Node{
		ip,
		client.Peers,
		tickInfo.Tick,
		time.Now().UTC().Unix(),
		true,
	}
	return &node, nil
}

func (n *Node) Update(connectionTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	client, err := qubic.NewClient(ctx, n.Address, nodePort)
	if err != nil {
		return errors.Wrap(err, "creating node connection")
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		n.LastUpdateSuccess = false
		return errors.Wrap(err, "getting tick info from node")
	}

	n.Peers = client.Peers
	n.LastTick = tickInfo.Tick
	n.LastUpdate = time.Now().UTC().Unix()
	n.LastUpdateSuccess = true

	return nil
}
