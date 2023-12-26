package types

import (
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/skip-mev/petri/provider"
)

type ChainI interface {
	GetConfig() ChainConfig
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
