package types

import (
	"context"
	"fmt"
	"math/big"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"google.golang.org/grpc"

	"github.com/skip-mev/petri/core/v3/provider"
)

// GenesisModifier is a function that takes in genesis bytes and returns modified genesis bytes
type GenesisModifier func([]byte) ([]byte, error)

// ChainI is an interface for a logical chain
type ChainI interface {
	Init(context.Context, ChainOptions) error
	Teardown(context.Context) error

	GetConfig() ChainConfig
	GetGRPCClient(context.Context) (*grpc.ClientConn, error)
	GetTMClient(context.Context) (*rpchttp.HTTP, error)

	GetValidators() []NodeI
	GetFaucetWallet() WalletI
	GetValidatorWallets() []WalletI

	GetNodes() []NodeI

	Height(context.Context) (uint64, error)
	WaitForBlocks(ctx context.Context, delta uint64) error
	WaitForHeight(ctx context.Context, desiredHeight uint64) error

	Serialize(ctx context.Context, p provider.ProviderI) ([]byte, error)
}

type ChainOptions struct {
	ModifyGenesis GenesisModifier // ModifyGenesis is a function that modifies the genesis bytes of the chain
	NodeOptions   NodeOptions     // NodeOptions is the options for creating a node
	NodeCreator   NodeCreator     // NodeCreator is a function that creates a node

	WalletConfig WalletConfig // WalletConfig is the default configuration of a chain's wallet
}

func (o ChainOptions) ValidateBasic() error {
	if err := o.WalletConfig.ValidateBasic(); err != nil {
		return fmt.Errorf("wallet config is invalid: %w", err)
	}

	if o.NodeCreator == nil {
		return fmt.Errorf("node creator cannot be nil")
	}

	return nil
}

// ChainConfig is the configuration structure for a logical chain.
// It contains all the relevant details needed to create a Cosmos chain
type ChainConfig struct {
	Name          string
	Denom         string // Denom is the denomination of the native staking token
	Decimals      uint64 // Decimals is the number of decimals of the native staking token
	NumValidators int    // NumValidators is the number of validators to create
	NumNodes      int    // NumNodes is the number of nodes to create

	BinaryName string   // BinaryName is the name of the chain binary in the Docker image
	Entrypoint []string // Entrypoint is the list of arguments to invoke in the entrypoint of the Docker image

	Image provider.ImageDefinition // Image is the Docker ImageDefinition of the chain

	GasPrices string // GasPrices are the minimum gas prices to set on the chain

	Bech32Prefix string // Bech32Prefix is the Bech32 prefix of the on-chain addresses

	HomeDir string // HomeDir is the home directory of the chain

	CoinType string // CoinType is the coin type of the chain (e.g. 118)
	ChainId  string // ChainId is the chain ID of the chain

	UseGenesisSubCommand bool     // UseGenesisSubCommand is a flag that indicates whether to use the 'genesis' subcommand to initialize the chain. Set to true if Cosmos SDK >v0.50
	AdditionalStartFlags []string // AdditionalStartFlags are additional flags to pass to the chain binary when starting the chain

	AdditionalPorts []string // AdditionalPorts are additional ports to expose for the chain

	// number of tokens to allocate per account in the genesis state (unscaled). This value defaults to 10_000_000 if not set.
	// if not set.
	GenesisDelegation *big.Int
	// number of tokens to allocate to the genesis account. This value defaults to 5_000_000 if not set.
	GenesisBalance *big.Int

	IsEVMChain            bool                   // IsEVMChain is used to set evm specific configs during chain creation
	CustomAppConfig       map[string]interface{} // CustomAppConfig is the configuration for the chain's app.toml
	CustomClientConfig    map[string]interface{} // CustomClientConfig is the configuration for the chain's client.toml
	CustomConsensusConfig map[string]interface{} // CustomConsensusConfig is the configuration for the chain's config.toml

	// SetPersistentPeers is used to determine whether nodes and validators of the network are added as persistent
	// peers to the consensus config
	SetPersistentPeers bool
	// SetPersistentPeers is used to determine whether a seed node is added to the consensus config
	SetSeedNode bool
}

func (c ChainConfig) GetGenesisBalance() *big.Int {
	if c.GenesisBalance == nil {
		return big.NewInt(10_000_000)
	}
	return c.GenesisBalance
}

func (c ChainConfig) GetGenesisDelegation() *big.Int {
	if c.GenesisDelegation == nil {
		return big.NewInt(5_000_000)
	}
	return c.GenesisDelegation
}

func (c ChainConfig) ValidateBasic() error {
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if c.Denom == "" {
		return fmt.Errorf("denom cannot be empty")
	}

	if c.Decimals == 0 {
		return fmt.Errorf("decimals cannot be 0")
	}

	if c.NumValidators == 0 {
		return fmt.Errorf("num validators cannot be 0")
	}

	if c.BinaryName == "" {
		return fmt.Errorf("binary name cannot be empty")
	}

	if c.GasPrices == "" {
		return fmt.Errorf("gas prices cannot be empty")
	}

	if err := c.Image.ValidateBasic(); err != nil {
		return fmt.Errorf("image definition is invalid: %w", err)
	}

	if c.Bech32Prefix == "" {
		return fmt.Errorf("bech32 prefix cannot be empty")
	}

	if c.CoinType == "" {
		return fmt.Errorf("coin type cannot be empty")
	}

	if c.ChainId == "" {
		return fmt.Errorf("chain ID cannot be empty")
	}

	return nil
}
