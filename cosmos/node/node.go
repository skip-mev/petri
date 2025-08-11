package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/skip-mev/petri/core/v3/provider"

	petritypes "github.com/skip-mev/petri/core/v3/types"
)

type PackagedState struct {
	State
	TaskState []byte
}

type State struct {
	Config      petritypes.NodeConfig
	ChainConfig petritypes.ChainConfig
}

type Node struct {
	provider.TaskI

	state  State
	logger *zap.Logger
}

var _ petritypes.NodeCreator = CreateNode
var _ petritypes.NodeRestorer = RestoreNode

// CreateNode creates a new logical node and creates the underlying workload for it
func CreateNode(ctx context.Context, logger *zap.Logger, infraProvider provider.ProviderI, nodeConfig petritypes.NodeConfig, opts petritypes.NodeOptions) (petritypes.NodeI, error) {
	if err := nodeConfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate node config: %w", err)
	}

	var node Node

	node.logger = logger.Named("node")
	chainConfig := nodeConfig.ChainConfig

	nodeState := State{
		Config:      nodeConfig,
		ChainConfig: chainConfig,
	}

	node.state = nodeState

	node.logger.Info("creating node", zap.String("name", nodeConfig.Name))

	def := provider.TaskDefinition{
		Name:        nodeConfig.Name,
		Image:       chainConfig.Image,
		Ports:       append([]string{"9090", "26656", "26657", "26660", "1317"}, chainConfig.AdditionalPorts...),
		Entrypoint:  append([]string{chainConfig.BinaryName, "--home", chainConfig.HomeDir, "start"}, chainConfig.AdditionalStartFlags...),
		DataDir:     chainConfig.HomeDir,
		Environment: map[string]string{"GODEBUG": "blockprofilerate=1"},
	}

	if opts.NodeDefinitionModifier != nil {
		def = opts.NodeDefinitionModifier(def, nodeConfig)
	}

	task, err := infraProvider.CreateTask(ctx, def)
	if err != nil {
		return nil, err
	}

	node.TaskI = task

	return &node, nil
}

func RestoreNode(ctx context.Context, logger *zap.Logger, state []byte, p provider.ProviderI) (petritypes.NodeI, error) {
	var packagedState PackagedState
	if err := json.Unmarshal(state, &packagedState); err != nil {
		return nil, fmt.Errorf("unmarshaling state: %w", err)
	}

	node := Node{
		state:  packagedState.State,
		logger: logger.Named("node"),
	}

	task, err := p.DeserializeTask(ctx, packagedState.TaskState)

	if err != nil {
		return nil, err
	}

	node.TaskI = task

	return &node, err
}

// GetTMClient returns a CometBFT HTTP client for the node
func (n *Node) GetTMClient(ctx context.Context) (*rpchttp.HTTP, error) {
	addr, err := n.GetExternalAddress(ctx, "26657")
	if err != nil {
		return nil, err
	}

	httpAddr := fmt.Sprintf("http://%s", addr)

	httpClient := &http.Client{
		Transport: &http.Transport{
			// Set to true to prevent GZIP-bomb DoS attacks
			DisableCompression: true,
			DialContext:        n.DialContext(),
			Proxy:              http.ProxyFromEnvironment,
		},
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
	cc, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return n.DialContext()(ctx, "tcp", addr)
		}),
	)

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
	command = append([]string{n.GetChainConfig().BinaryName}, command...)
	return append(command,
		"--home", n.state.ChainConfig.HomeDir,
	)
}

// GetConfig returns the node's config
func (n *Node) GetConfig() petritypes.NodeConfig {
	return n.state.Config
}

func (n *Node) GetChainConfig() petritypes.ChainConfig {
	return n.state.ChainConfig
}

func (n *Node) Serialize(ctx context.Context, p provider.ProviderI) ([]byte, error) {
	taskState, err := p.SerializeTask(ctx, n.TaskI)

	if err != nil {
		return nil, err
	}

	state := PackagedState{
		State:     n.state,
		TaskState: taskState,
	}

	return json.Marshal(state)
}
