package types

import (
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/skip-mev/petri/provider"
	"google.golang.org/grpc"
)

type ChainI interface {
	GetConfig() ChainConfig
	GetGPRCClient() (*grpc.ClientConn, error)
	GetTMClient() (rpcclient.Client, error)
	GetTxConfig() client.TxConfig
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
	ChainId  string

	ModifyGenesis GenesisModifier

	NodeCreator NodeCreator
}

type GenesisModifier func([]byte) ([]byte, error)
