package node

import (
	"context"
	"github.com/pkg/errors"
	qubic "github.com/qubic/go-node-connector"
	"github.com/qubic/go-node-connector/types"
	"log"
	"time"
)

type Node struct {
	Address           string
	Port              string
	Peers             types.PublicPeers
	LastTick          uint32
	LastUpdate        int64
	LastUpdateSuccess bool
}

func NewNode(ip string, port string, connectionTimeout time.Duration) (*Node, error) {

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	client, err := qubic.NewClient(ctx, ip, port)
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
		Address:           ip,
		Port:              port,
		Peers:             client.Peers,
		LastTick:          tickInfo.Tick,
		LastUpdate:        time.Now().UTC().Unix(),
		LastUpdateSuccess: true,
	}
	return &node, nil
}

func (n *Node) Update(connectionTimeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	client, err := qubic.NewClient(ctx, n.Address, n.Port)
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
