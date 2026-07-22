package node

import (
	"context"
	"fmt"
	"log"
	"time"

	qubic "github.com/qubic/go-node-connector/v2"
	"github.com/qubic/go-node-connector/v2/types"
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
		return nil, fmt.Errorf("creating node connection: %w", err)
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting tick info from node: %w", err)
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
		return fmt.Errorf("creating node connection: %w", err)
	}
	defer client.Close()

	tickInfo, err := client.GetTickInfo(ctx)
	if err != nil {
		n.LastUpdateSuccess = false
		return fmt.Errorf("getting tick info from node: %w", err)
	}

	n.Peers = client.Peers
	n.LastTick = tickInfo.Tick
	n.LastUpdate = time.Now().UTC().Unix()
	n.LastUpdateSuccess = true

	return nil
}
