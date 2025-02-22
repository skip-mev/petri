package types

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"

	"github.com/skip-mev/petri/core/v3/provider"
)

// NodeOptions is a struct that contains the options for creating a node
type NodeOptions struct {
	NodeDefinitionModifier NodeDefinitionModifier // NodeDefinitionModifier is a function that modifies a node's definition
}

// NodeConfig is the configuration structure for a logical node.
type NodeConfig struct {
	Name  string // Name is the name of the node
	Index int    // Index denotes which node this is in the Validators/Nodes array

	IsValidator bool // IsValidator denotes whether this node is a validator

	ChainConfig ChainConfig // ChainConfig is the config of the chain this node is running on
}

func (c NodeConfig) ValidateBasic() error {
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if err := c.ChainConfig.ValidateBasic(); err != nil {
		return err
	}

	return nil
}

// NodeDefinitionModifier is a type of function that given a NodeConfig modifies the task definition. It usually
// modifies the entrypoint. This function is typically called in NodeCreator
// before the task is created
type NodeDefinitionModifier func(provider.TaskDefinition, NodeConfig) provider.TaskDefinition

// NodeCreator is a type of function that given a NodeConfig creates a new logical node
type NodeCreator func(context.Context, *zap.Logger, provider.ProviderI, NodeConfig, NodeOptions) (NodeI, error)

// NodeRestorer is a type of function that given a NodeState restores a logical node
type NodeRestorer func(context.Context, *zap.Logger, []byte, provider.ProviderI) (NodeI, error)

// NodeI represents an interface for a  logical node that is running on a chain
type NodeI interface {
	provider.TaskI

	// GetConfig returns the configuration of the node
	GetConfig() NodeConfig

	// GetTMClient returns the CometBFT RPC client of the node
	GetTMClient(context.Context) (*rpchttp.HTTP, error)
	// GetGRPCClient returns the gRPC client of the node
	GetGRPCClient(context.Context) (*grpc.ClientConn, error)

	// Height returns the current height of the node
	Height(context.Context) (uint64, error)

	// InitHome creates a home directory on the node
	InitHome(context.Context) error

	// AddGenesisAccount adds a genesis account to the node's genesis file
	AddGenesisAccount(context.Context, string, []sdk.Coin) error
	// GenerateGenTx creates a genesis transaction using the validator's key on the node
	GenerateGenTx(context.Context, sdk.Coin) error
	// CopyGenTx copies the genesis transaction to another node
	CopyGenTx(context.Context, NodeI) error
	// CollectGenTxs collects all of the genesis transactions on the node and creates the genesis file
	CollectGenTxs(context.Context) error

	// GenesisFileContent returns the contents of the genesis file on the node
	GenesisFileContent(context.Context) ([]byte, error)
	// OverwriteGenesisFile overwrites the genesis file on the node with the given contents
	OverwriteGenesisFile(context.Context, []byte) error

	// CreateWallet creates a Cosmos wallet on the node
	CreateWallet(context.Context, string, WalletConfig) (WalletI, error)
	// RecoverKey creates a Cosmos wallet on the node given a mnemonic
	RecoverKey(context.Context, string, string) error
	// KeyBech32 returns the Bech32 address of a key on the node
	KeyBech32(context.Context, string, string) (string, error)

	// SetDefaultConfigs sets the default configurations for the app and consensus of the node
	SetDefaultConfigs(context.Context) error
	// SetPersistentPeers takes in a comma-delimited peer string (nodeid1@host1:port1,nodeid2@host2:port2) and writes it
	// to the consensus config file on the node
	SetPersistentPeers(context.Context, string) error

	// NodeId returns the p2p peer ID of the node
	NodeId(context.Context) (string, error)

	// GetDefinition returns the task definition of the node
	GetDefinition() provider.TaskDefinition

	// GetIP returns the IP address of the node
	GetIP(context.Context) (string, error)

	// Serialize serializes the node
	Serialize(context.Context, provider.ProviderI) ([]byte, error)
}
