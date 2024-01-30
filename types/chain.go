package types

import (
	"context"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/skip-mev/petri/provider/v2"
	"google.golang.org/grpc"
)

// ChainI is an interface for a logical chain
type ChainI interface {
	Init(context.Context) error
	Teardown(context.Context) error

	GetConfig() ChainConfig
	GetGRPCClient(context.Context) (*grpc.ClientConn, error)
	GetTMClient(context.Context) (*rpchttp.HTTP, error)
	GetTxConfig() client.TxConfig
	GetInterfaceRegistry() codectypes.InterfaceRegistry

	GetValidators() []NodeI
	GetFaucetWallet() WalletI
	GetValidatorWallets() []WalletI

	GetNodes() []NodeI

	Height(context.Context) (uint64, error)
	WaitForBlocks(ctx context.Context, delta uint64) error
	WaitForHeight(ctx context.Context, desiredHeight uint64) error
}

// ChainConfig is the configuration structure for a logical chain.
// It contains all the relevant details needed to create a Cosmos chain and it's sidecars
type ChainConfig struct {
	Denom         string // Denom is the denomination of the native staking token
	Decimals      uint64 // Decimals is the number of decimals of the native staking token
	NumValidators int    // NumValidators is the number of validators to create
	NumNodes      int    // NumNodes is the number of nodes to create

	BinaryName string // BinaryName is the name of the chain binary in the Docker image

	Image        provider.ImageDefinition // Image is the Docker ImageDefinition of the chain
	SidecarImage provider.ImageDefinition // SidecarImage is the Docker ImageDefinition of the chain sidecar

	GasPrices     string  // GasPrices are the minimum gas prices to set on the chain
	GasAdjustment float64 // GasAdjustment is the margin by which to multiply the default gas prices

	Bech32Prefix string // Bech32Prefix is the Bech32 prefix of the on-chain addresses

	EncodingConfig testutil.TestEncodingConfig // EncodingConfig is the encoding config of the chain

	HomeDir        string   // HomeDir is the home directory of the chain
	SidecarHomeDir string   // SidecarHomeDir is the home directory of the chain sidecar
	SidecarPorts   []string // SidecarPorts are the ports to expose on the chain sidecar
	SidecarArgs    []string // SidecarArgs are the arguments to launch the chain sidecar

	CoinType string // CoinType is the coin type of the chain (e.g. 118)
	HDPath   string // HDPath is the HD path of the chain (e.g. m/44'/118'/0'/0/0)
	ChainId  string // ChainId is the chain ID of the chain

	ModifyGenesis GenesisModifier // ModifyGenesis is a function that modifies the genesis bytes of the chain

	WalletConfig WalletConfig // WalletConfig is the default configuration of a chain's wallet

	UseGenesisSubCommand bool // UseGenesisSubCommand is a flag that indicates whether to use the 'genesis' subcommand to initialize the chain. Set to true if Cosmos SDK >v0.50

	NodeCreator            NodeCreator            // NodeCreator is a function that creates a node
	NodeDefinitionModifier NodeDefinitionModifier // NodeDefinitionModifier is a function that modifies a node's definition
}

// GenesisModifier is a function that takes in genesis bytes and returns modified genesis bytes
type GenesisModifier func([]byte) ([]byte, error)
