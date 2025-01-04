package node

import (
	"context"
	"fmt"
	"time"

	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/skip-mev/petri/core/v2/provider"

	petritypes "github.com/skip-mev/petri/core/v2/types"
)

type Node struct {
	provider.TaskI

	logger *zap.Logger
	config petritypes.NodeConfig
	chain  petritypes.ChainI
}

var _ petritypes.NodeCreator = CreateNode

// CreateNode creates a new logical node and creates the underlying workload for it
func CreateNode(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, nodeConfig petritypes.NodeConfig) (petritypes.NodeI, error) {
	if err := nodeConfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate node config: %w", err)
	}

	var node Node

	node.logger = logger.Named("node")
	node.chain = nodeConfig.Chain
	node.config = nodeConfig

	node.logger.Info("creating node", zap.String("name", nodeConfig.Name))

	chainConfig := nodeConfig.Chain.GetConfig()

	def := provider.TaskDefinition{
		Name:          nodeConfig.Name,
		ContainerName: nodeConfig.Name,
		Image:         chainConfig.Image,
		Ports:         []string{"9090", "26656", "26657", "26660", "80"},
		Entrypoint:    []string{chainConfig.BinaryName, "--home", chainConfig.HomeDir, "start"},
		DataDir:       chainConfig.HomeDir,
	}

	if nodeConfig.Chain.GetConfig().NodeDefinitionModifier != nil {
		def = nodeConfig.Chain.GetConfig().NodeDefinitionModifier(def, nodeConfig)
	}

	task, err := infraProvider.CreateTask(ctx, def)
	if err != nil {
		return nil, err
	}

	node.TaskI = task

	return &node, nil
}

// GetTMClient returns a CometBFT HTTP client for the node
func (n *Node) GetTMClient(ctx context.Context) (*rpchttp.HTTP, error) {
	addr, err := n.GetExternalAddress(ctx, "26657")
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

// GetGRPCClient returns a GRPC client for the node
func (n *Node) GetGRPCClient(ctx context.Context) (*grpc.ClientConn, error) {
	grpcAddr, err := n.GetExternalAddress(ctx, "9090")
	if err != nil {
		return nil, err
	}

	// create the client
	cc, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return cc, nil
}

// Height returns the current block height of the node
func (n *Node) Height(ctx context.Context) (uint64, error) {
	n.logger.Debug("getting height", zap.String("node", n.GetDefinition().Name))
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

// NodeId returns the node's p2p ID
func (n *Node) NodeId(ctx context.Context) (string, error) {
	j, err := n.ReadFile(ctx, "config/node_key.json")
	if err != nil {
		return "", fmt.Errorf("getting node_key.json content: %w", err)
	}

	var nk p2p.NodeKey
	if err := tmjson.Unmarshal(j, &nk); err != nil {
		return "", fmt.Errorf("unmarshaling node_key.json: %w", err)
	}

	return string(nk.ID()), nil
}

// BinCommand returns a command that can be used to run a binary on the node
func (n *Node) BinCommand(command ...string) []string {
	chainConfig := n.chain.GetConfig()

	command = append([]string{chainConfig.BinaryName}, command...)
	return append(command,
		"--home", chainConfig.HomeDir,
	)
}

// GetConfig returns the node's config
func (n *Node) GetConfig() petritypes.NodeConfig {
	return n.config
}
