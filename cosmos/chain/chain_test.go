package chain_test

import (
	"context"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/docker"
	"github.com/skip-mev/petri/core/v2/types"
	"github.com/skip-mev/petri/cosmos/v2/chain"
	"github.com/skip-mev/petri/cosmos/v2/node"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
	"time"
)

const idAlphabet = "abcdefghijklqmnoqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var defaultChainConfig = types.ChainConfig{
	Denom:         "stake",
	Decimals:      6,
	NumValidators: 1,
	NumNodes:      0,
	BinaryName:    "/simd/simd",
	Image: provider.ImageDefinition{
		Image: "cosmossdk/simd:latest",
		UID:   "1000",
		GID:   "1000",
	},
	GasPrices:    "0.0005stake",
	Bech32Prefix: "cosmos",
	HomeDir:      "/gaia",
	CoinType:     "118",
	ChainId:      "stake-1",
	WalletConfig: types.WalletConfig{
		SigningAlgorithm: string(hd.Secp256k1.Name()),
		Bech32Prefix:     "cosmos",
		HDPath:           hd.CreateHDPath(118, 0, 0),
		DerivationFn:     hd.Secp256k1.Derive(),
		GenerationFn:     hd.Secp256k1.Generate(),
	},
	UseGenesisSubCommand: true,
	NodeCreator:          node.CreateNode,
}

func TestChainLifecycle(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		require.NoError(t, p.Teardown(ctx))
	}(p, ctx)

	c, err := chain.CreateChain(ctx, logger, p, defaultChainConfig)
	require.NoError(t, err)

	require.NoError(t, c.Init(ctx))
	require.Len(t, c.GetValidators(), 1)
	require.Len(t, c.GetNodes(), 0)

	time.Sleep(1 * time.Second)

	require.NoError(t, c.WaitForBlocks(ctx, 5))

	require.NoError(t, c.Teardown(ctx))
}
