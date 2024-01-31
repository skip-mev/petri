package node

import (
	"context"
	"fmt"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	libclient "github.com/cometbft/cometbft/rpc/jsonrpc/client"
	"github.com/skip-mev/petri/core/v2/provider"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"

	petritypes "github.com/skip-mev/petri/core/v2/types"
)

type Node struct {
	*provider.Task

	logger *zap.Logger
	config petritypes.NodeConfig
	chain  petritypes.ChainI
}

var _ petritypes.NodeCreator = CreateNode

func CreateNode(ctx context.Context, logger *zap.Logger, nodeConfig petritypes.NodeConfig) (petritypes.NodeI, error) {
	var node Node

	node.logger = logger.Named("node")
	node.chain = nodeConfig.Chain
	node.config = nodeConfig

	node.logger.Info("creating node", zap.String("name", nodeConfig.Name))

	chainConfig := nodeConfig.Chain.GetConfig()

	var sidecars []provider.TaskDefinition

	if chainConfig.SidecarHomeDir != "" {
		sidecars = append(sidecars, provider.TaskDefinition{
			Name:          fmt.Sprintf("%s-sidecar-%d", nodeConfig.Name, 0), // todo(Zygimantass): fix this to support multiple sidecars
			ContainerName: fmt.Sprintf("%s-sidecar-%d", nodeConfig.Name, 0),
			Image:         chainConfig.SidecarImage,
			DataDir:       chainConfig.SidecarHomeDir,
			Ports:         chainConfig.SidecarPorts,
			Entrypoint:    chainConfig.SidecarArgs,
		})
	}

	def := provider.TaskDefinition{
		Name:          nodeConfig.Name,
		ContainerName: nodeConfig.Name,
		Image:         chainConfig.Image,
		Ports:         []string{"9090", "26656", "26657", "26660", "80"},
		Sidecars:      sidecars,
		Entrypoint:    []string{chainConfig.BinaryName, "--home", chainConfig.HomeDir, "start"},
		DataDir:       chainConfig.HomeDir,
	}

	if nodeConfig.Chain.GetConfig().NodeDefinitionModifier != nil {
		def = nodeConfig.Chain.GetConfig().NodeDefinitionModifier(def, nodeConfig)
	}

	task, err := provider.CreateTask(ctx, node.logger, nodeConfig.Provider, def)

	if err != nil {
		return nil, err
	}

	node.Task = task

	return &node, nil
}

func (n *Node) GetTask() *provider.Task {
	return n.Task
}

func (n *Node) GetTMClient(ctx context.Context) (*rpchttp.HTTP, error) {
	addr, err := n.Task.GetExternalAddress(ctx, "26657")

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
	grpcAddr, err := n.GetExternalAddress(ctx, "9090")
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
	n.logger.Debug("getting height", zap.String("node", n.Definition.Name))
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
	chainConfig := n.chain.GetConfig()

	command = append([]string{chainConfig.BinaryName}, command...)
	return append(command,
		"--home", chainConfig.HomeDir,
	)
}

func (n *Node) GetConfig() petritypes.NodeConfig {
	return n.config
}
