package node

import (
	"context"
	"github.com/pkg/errors"
	qubic "github.com/qubic/go-node-connector"
	"time"
)

func NewNodeConnection(address string, timeout time.Duration) (*qubic.Client, context.Context, context.CancelFunc, error) {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	client, err := qubic.NewClient(ctx, address, nodePort)
	if err != nil {
		return nil, ctx, cancel, errors.Wrap(err, "getting client")
	}
	return client, ctx, cancel, nil

}
