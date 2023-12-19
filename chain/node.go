package chain

import (
	"context"
	"fmt"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/skip-mev/petri/provider"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

const (
	validatorKey = "validator"
)

type Node struct {
	*provider.Task

	isValidator bool
	chain       *Chain
}

func (n *Node) GetTMClient(ctx context.Context) (rpcclient.Client, error) {
	addr, err := n.Task.GetExternalAddress(ctx, "26657/tcp")

	if err != nil {
		panic(err)
	}

	httpAddr := fmt.Sprintf("http://%s", addr)

	httpClient, err := libclient.DefaultHTTPClient(httpAddr)
	if err != nil {
		return nil, err
	}

	httpClient.Timeout = 10 * time.Second
	rpcClient, err := rpchttp.NewWithClient(httpAddr, "/websocket", httpClient)
	if err != nil {
		return nil, err
	}

	return rpcClient, nil
}

func (n *Node) GetGRPCClient(ctx context.Context) (*grpc.ClientConn, error) {
	grpcAddr, err := n.GetExternalAddress(ctx, "9090/tcp")
	if err != nil {
		return nil, err
	}

	// create the client
	cc, err := grpc.Dial(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return cc, nil
}

func (n *Node) Height(ctx context.Context) (uint64, error) {
	client, err := n.GetTMClient(ctx)

	if err != nil {
		return 0, err
	}

	block, err := client.Block(ctx, nil)

	if err != nil {
		return 0, err
	}

	return uint64(block.Block.Height), nil
}

func (n *Node) NodeId(ctx context.Context) (string, error) {
	// This used to call p2p.LoadNodeKey against the file on the host,
	// but because we are transitioning to operating on Docker volumes,
	// we only have to tmjson.Unmarshal the raw content.
	j, err := n.Task.ReadFile(ctx, "config/node_key.json")
	if err != nil {
		return "", fmt.Errorf("getting node_key.json content: %w", err)
	}

	var nk p2p.NodeKey
	if err := tmjson.Unmarshal(j, &nk); err != nil {
		return "", fmt.Errorf("unmarshaling node_key.json: %w", err)
	}

	return string(nk.ID()), nil
}

func (n *Node) BinCommand(command ...string) []string {
	command = append([]string{n.chain.Config.BinaryName}, command...)
	return append(command,
		"--home", n.chain.Config.HomeDir,
	)
}

func (n *Node) NodeCommand(command ...string) ([]string, error) {
	command = n.BinCommand(command...)
	ip, err := n.GetIP(context.Background())

	if err != nil {
		return []string{}, nil
	}

	return append(command,
		"--node", fmt.Sprintf("tcp://%s:26657", ip),
	), nil
}