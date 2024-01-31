package types

import (
	"context"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
<<<<<<< HEAD:general/types/chain.go
<<<<<<< HEAD:types/chain.go
	"github.com/skip-mev/petri/provider"
=======
	"github.com/skip-mev/petri/general/v2/provider"
>>>>>>> cd1f05b (chore: move everything inside of two packages):general/types/chain.go
=======
	"github.com/skip-mev/petri/core/v2/provider"
>>>>>>> d34ae41 (fix: general -> core):core/types/chain.go
	"google.golang.org/grpc"
)

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

type ChainConfig struct {
	Denom         string
	Decimals      uint64
	NumValidators int
	NumNodes      int

	BinaryName string

	Image        provider.ImageDefinition
	SidecarImage provider.ImageDefinition

	GasPrices     string
	GasAdjustment float64

	Bech32Prefix string

	EncodingConfig testutil.TestEncodingConfig

	HomeDir        string
	SidecarHomeDir string
	SidecarPorts   []string
	SidecarArgs    []string

	CoinType string
	HDPath   string
	ChainId  string

	ModifyGenesis GenesisModifier

	WalletConfig WalletConfig

	UseGenesisSubCommand bool

	NodeCreator NodeCreator
}

type GenesisModifier func([]byte) ([]byte, error)
